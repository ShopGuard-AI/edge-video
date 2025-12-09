#!/usr/bin/env python3
"""
Edge Video Consumer - Production Ready
Consome frames do RabbitMQ e processa com Redis fallback
"""

import os
import sys
import time
import signal
import logging
import threading
from datetime import datetime
from typing import Optional, Dict, Any
from concurrent.futures import ThreadPoolExecutor, as_completed

import pika
import redis
import yaml
import cv2
import numpy as np
from prometheus_client import Counter, Histogram, Gauge, start_http_server

# ============================================================================
# LOGGING SETUP
# ============================================================================
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y/%m/%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

# ============================================================================
# PROMETHEUS METRICS
# ============================================================================
frames_consumed_total = Counter(
    'consumer_frames_consumed_total',
    'Total frames consumed from RabbitMQ',
    ['camera_id', 'status']
)

frames_processing_duration = Histogram(
    'consumer_frames_processing_duration_seconds',
    'Time to process each frame',
    ['camera_id'],
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0]
)

redis_fetch_total = Counter(
    'consumer_redis_fetch_total',
    'Total Redis fetch operations',
    ['status']
)

redis_fetch_duration = Histogram(
    'consumer_redis_fetch_duration_seconds',
    'Time to fetch frame from Redis',
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0]
)

rabbitmq_connection_status = Gauge(
    'consumer_rabbitmq_connected',
    'RabbitMQ connection status (1=connected, 0=disconnected)'
)

redis_connection_status = Gauge(
    'consumer_redis_connected',
    'Redis connection status (1=connected, 0=disconnected)'
)

frames_in_queue = Gauge(
    'consumer_frames_in_queue',
    'Approximate number of frames in queue'
)


# ============================================================================
# CONSUMER CLASS
# ============================================================================
class EdgeVideoConsumer:
    """
    High-performance RabbitMQ consumer with Redis fallback
    """

    def __init__(self, config_path: str = "config.yaml"):
        """Initialize consumer with configuration"""
        self.config = self._load_config(config_path)
        self.running = False
        self.connection: Optional[pika.BlockingConnection] = None
        self.channel: Optional[pika.channel.Channel] = None
        self.redis_client: Optional[redis.Redis] = None

        # Statistics
        self.stats = {
            'total_consumed': 0,
            'total_success': 0,
            'total_errors': 0,
            'start_time': None,
            'camera_counts': {},
            'fps_counter': 0,
            'fps_start_time': time.time(),
            'current_fps': 0.0,
            'redis_hits': 0,
            'redis_misses': 0,
            'avg_processing_time': 0.0
        }

        # Exibir apenas 1 câmera (a primeira que chegar)
        self.selected_camera = None

        # Thread pool for parallel processing
        self.executor = ThreadPoolExecutor(
            max_workers=self.config['processing']['max_workers']
        )

        # Graceful shutdown
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)

    def _load_config(self, path: str) -> Dict[str, Any]:
        """Load configuration from YAML file"""
        with open(path, 'r') as f:
            return yaml.safe_load(f)

    def _signal_handler(self, signum, frame):
        """Handle graceful shutdown on SIGINT/SIGTERM"""
        logger.info(f"Received signal {signum}, initiating graceful shutdown...")
        self.stop()

    def _connect_redis(self):
        """Connect to Redis"""
        if not self.config['redis']['enabled']:
            logger.info("Redis DISABLED - skipping connection")
            return

        try:
            self.redis_client = redis.Redis(
                host=self.config['redis']['address'].split(':')[0],
                port=int(self.config['redis']['address'].split(':')[1]),
                password=self.config['redis']['password'],
                db=self.config['redis']['db'],
                socket_timeout=self.config['redis']['timeout'],
                socket_connect_timeout=self.config['redis']['timeout'],
                decode_responses=False  # Keep binary data
            )

            # Test connection
            self.redis_client.ping()
            logger.info(f"✓ Redis connected: {self.config['redis']['address']}")
            redis_connection_status.set(1)

        except Exception as e:
            logger.error(f"✗ Redis connection failed: {e}")
            self.redis_client = None
            redis_connection_status.set(0)

    def _connect_rabbitmq(self):
        """Connect to RabbitMQ with retry"""
        max_retries = 10
        retry_delay = 5

        for attempt in range(1, max_retries + 1):
            try:
                logger.info(f"Connecting to RabbitMQ (attempt {attempt}/{max_retries})...")

                # Parse connection parameters
                params = pika.URLParameters(self.config['rabbitmq']['url'])
                params.heartbeat = 30
                params.blocked_connection_timeout = 300

                # Connect
                self.connection = pika.BlockingConnection(params)
                self.channel = self.connection.channel()

                # Set QoS (prefetch)
                self.channel.basic_qos(
                    prefetch_count=self.config['rabbitmq']['prefetch_count']
                )

                # Declare exchange (idempotent)
                self.channel.exchange_declare(
                    exchange=self.config['rabbitmq']['exchange'],
                    exchange_type='topic',
                    durable=True
                )

                # Declare queue (idempotent)
                queue_result = self.channel.queue_declare(
                    queue=self.config['rabbitmq']['queue'],
                    durable=True,
                    arguments={
                        'x-max-length': 1,  # Max 1 mensagem (apenas frame mais recente)
                        'x-overflow': 'drop-head'  # Descarta mensagem antiga quando nova chega
                    }
                )

                # Bind queue to exchange
                self.channel.queue_bind(
                    exchange=self.config['rabbitmq']['exchange'],
                    queue=self.config['rabbitmq']['queue'],
                    routing_key=self.config['rabbitmq']['routing_key']
                )

                logger.info(f"✓ RabbitMQ connected - Queue: {self.config['rabbitmq']['queue']}")
                logger.info(f"✓ QoS: prefetch_count={self.config['rabbitmq']['prefetch_count']}")
                logger.info(f"✓ Messages in queue: {queue_result.method.message_count}")

                frames_in_queue.set(queue_result.method.message_count)
                rabbitmq_connection_status.set(1)

                return

            except Exception as e:
                logger.error(f"Connection attempt {attempt} failed: {e}")
                rabbitmq_connection_status.set(0)

                if attempt < max_retries:
                    logger.info(f"Retrying in {retry_delay} seconds...")
                    time.sleep(retry_delay)
                else:
                    raise RuntimeError(f"Failed to connect after {max_retries} attempts")

    def _fetch_from_redis(self, redis_key: str) -> Optional[bytes]:
        """Fetch frame from Redis by key"""
        if not self.redis_client:
            return None

        start = time.time()
        try:
            frame_data = self.redis_client.get(redis_key)
            duration = time.time() - start

            redis_fetch_duration.observe(duration)

            if frame_data:
                redis_fetch_total.labels(status='success').inc()
                return frame_data
            else:
                redis_fetch_total.labels(status='not_found').inc()
                logger.warning(f"Frame not found in Redis: {redis_key}")
                return None

        except Exception as e:
            duration = time.time() - start
            redis_fetch_duration.observe(duration)
            redis_fetch_total.labels(status='error').inc()
            logger.error(f"Redis fetch error: {e}")
            return None

    def _process_frame(self, camera_id: str, frame_data: bytes, headers: Dict) -> bool:
        """
        Process a single frame - EXIBE NA TELA usando OpenCV com MÉTRICAS
        OTIMIZADO: Só processa frames da câmera selecionada!
        Returns True on success, False on error
        """
        start = time.time()

        try:
            # Seleciona a primeira câmera que chegar
            if self.selected_camera is None:
                self.selected_camera = camera_id
                logger.info(f"📹 Exibindo frames da câmera: {camera_id}")

            # OTIMIZAÇÃO CRÍTICA: Pula frames de outras câmeras SEM processar!
            if camera_id != self.selected_camera:
                # Apenas valida rapidamente e retorna (sem decodificar!)
                if frame_data.startswith(b'\xff\xd8') and frame_data.endswith(b'\xff\xd9'):
                    return True
                return False

            # Valida se é JPEG válido
            if not (frame_data.startswith(b'\xff\xd8') and frame_data.endswith(b'\xff\xd9')):
                logger.warning(f"[{camera_id}] Invalid JPEG data (size: {len(frame_data)} bytes)")
                return False

            # Decodifica JPEG para numpy array
            nparr = np.frombuffer(frame_data, np.uint8)
            frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

            if frame is None:
                logger.warning(f"[{camera_id}] Failed to decode frame")
                return False

            # Calcula FPS
            self.stats['fps_counter'] += 1
            elapsed = time.time() - self.stats['fps_start_time']
            if elapsed >= 1.0:  # Atualiza FPS a cada 1 segundo
                self.stats['current_fps'] = self.stats['fps_counter'] / elapsed
                self.stats['fps_counter'] = 0
                self.stats['fps_start_time'] = time.time()

            # Calcula tempo médio de processamento
            duration_ms = (time.time() - start) * 1000
            self.stats['avg_processing_time'] = (
                (self.stats['avg_processing_time'] * 0.9) + (duration_ms * 0.1)
            )

            # Dimensões do frame
            height, width = frame.shape[:2]
            timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

            # Taxa de acerto Redis
            total_redis = self.stats['redis_hits'] + self.stats['redis_misses']
            redis_hit_rate = (self.stats['redis_hits'] / total_redis * 100) if total_redis > 0 else 0

            # Uptime
            uptime_sec = time.time() - self.stats['start_time'] if self.stats['start_time'] else 0
            uptime_str = f"{int(uptime_sec//3600):02d}:{int((uptime_sec%3600)//60):02d}:{int(uptime_sec%60):02d}"

            # ========== DESENHA MÉTRICAS NO FRAME ==========

            # Painel superior (fundo semi-transparente)
            overlay = frame.copy()
            cv2.rectangle(overlay, (0, 0), (width, 180), (0, 0, 0), -1)
            cv2.addWeighted(overlay, 0.7, frame, 0.3, 0, frame)

            # Linha 1: Câmera e Timestamp
            cv2.putText(frame, f"CAMERA: {camera_id.upper()}", (10, 30),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 0), 2)
            cv2.putText(frame, timestamp, (10, 60),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 1)

            # Linha 2: FPS e Frames processados
            fps_color = (0, 255, 0) if self.stats['current_fps'] > 10 else (0, 165, 255)
            cv2.putText(frame, f"FPS: {self.stats['current_fps']:.1f}", (10, 95),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.7, fps_color, 2)
            cv2.putText(frame, f"| Frames: {self.stats['total_success']}", (150, 95),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 1)

            # Linha 3: Processing time e Uptime
            cv2.putText(frame, f"Proc: {self.stats['avg_processing_time']:.1f}ms", (10, 125),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (100, 200, 255), 1)
            cv2.putText(frame, f"| Uptime: {uptime_str}", (200, 125),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 1)

            # Linha 4: Redis hit rate e Errors
            redis_color = (0, 255, 0) if redis_hit_rate > 50 else (0, 165, 255)
            cv2.putText(frame, f"Redis: {redis_hit_rate:.0f}%", (10, 155),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, redis_color, 1)
            error_rate = (self.stats['total_errors'] / self.stats['total_consumed'] * 100) if self.stats['total_consumed'] > 0 else 0
            error_color = (0, 255, 0) if error_rate < 1 else (0, 0, 255)
            cv2.putText(frame, f"| Errors: {self.stats['total_errors']} ({error_rate:.1f}%)", (180, 155),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, error_color, 1)

            # Painel inferior: Contadores por câmera
            panel_y = height - 100
            overlay2 = frame.copy()
            cv2.rectangle(overlay2, (0, panel_y), (width, height), (0, 0, 0), -1)
            cv2.addWeighted(overlay2, 0.7, frame, 0.3, 0, frame)

            cv2.putText(frame, "CAMERAS:", (10, panel_y + 25),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.5, (255, 255, 0), 1)

            x_offset = 120
            for cam, count in sorted(self.stats['camera_counts'].items()):
                cam_color = (0, 255, 0) if cam == self.selected_camera else (150, 150, 150)
                cv2.putText(frame, f"{cam}: {count}", (x_offset, panel_y + 25),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.5, cam_color, 1)
                x_offset += 120

            # EXIBE O FRAME na janela
            window_name = "Edge Video Consumer - Live View"
            cv2.imshow(window_name, frame)

            # OTIMIZAÇÃO: cv2.waitKey só a cada 2 frames (reduz bloqueio no Windows)
            if self.stats['fps_counter'] % 2 == 0:
                cv2.waitKey(1)  # Atualiza a janela

            # Opcionalmente, salva frame em disco
            if self.config['processing']['save_frames']:
                output_dir = self.config['processing']['output_dir']
                os.makedirs(f"{output_dir}/{camera_id}", exist_ok=True)

                timestamp_file = datetime.now().strftime("%Y%m%d_%H%M%S_%f")
                filename = f"{output_dir}/{camera_id}/frame_{timestamp_file}.jpg"

                with open(filename, 'wb') as f:
                    f.write(frame_data)

            # Sucesso!
            duration = time.time() - start
            frames_processing_duration.labels(camera_id=camera_id).observe(duration)

            return True

        except Exception as e:
            logger.error(f"[{camera_id}] Processing error: {e}")
            return False

    def _process_frame_worker(self, camera_id, frame_data, headers):
        """
        Worker thread para processar frame ASSINCRONAMENTE
        CRITICAL: NÃO faz ACK/NACK aqui (thread-safety!)
        Retorna success/failure para callback fazer ACK/NACK na thread principal
        """
        try:
            # Processa frame (decodifica + exibe + métricas)
            success = self._process_frame(camera_id, frame_data, headers)

            if success:
                self.stats['total_success'] += 1
                frames_consumed_total.labels(camera_id=camera_id, status='success').inc()
            else:
                self.stats['total_errors'] += 1
                frames_consumed_total.labels(camera_id=camera_id, status='error').inc()

            return success

        except Exception as e:
            logger.error(f"[{camera_id}] Worker thread error: {e}")
            self.stats['total_errors'] += 1
            frames_consumed_total.labels(camera_id=camera_id, status='error').inc()
            return False

    def _callback(self, ch, method, properties, body):
        """
        RabbitMQ message callback - SÍNCRONO mas OTIMIZADO
        CRITICAL: ACK/NACK deve ficar aqui (thread-safety do pika)
        """
        camera_id = "unknown"

        try:
            # Extract headers
            headers = properties.headers or {}
            camera_id = headers.get('camera_id', 'unknown')
            redis_key = headers.get('redis_key', '')

            # Update stats
            self.stats['total_consumed'] += 1
            self.stats['camera_counts'][camera_id] = \
                self.stats['camera_counts'].get(camera_id, 0) + 1

            # Log progress
            if self.stats['total_consumed'] % self.config['processing']['log_every_n'] == 0:
                elapsed = time.time() - self.stats['start_time']
                fps = self.stats['total_consumed'] / elapsed if elapsed > 0 else 0
                logger.info(f"Processed {self.stats['total_consumed']} frames "
                           f"({fps:.1f} fps) - Success: {self.stats['total_success']}, "
                           f"Errors: {self.stats['total_errors']}")

            # V1.6-STYLE: Redis é OBRIGATÓRIO
            # Body agora contém apenas redis_key (string), NÃO o frame
            frame_data = None

            # 1. Extrai redis_key do body (V1.6 style)
            if not redis_key:
                # Body agora é o redis_key (ex: "frames:cam1:1733681234567")
                redis_key = body.decode('utf-8') if isinstance(body, bytes) else body

            # 2. Valida que Redis está habilitado (obrigatório)
            if not self.redis_client:
                logger.error(f"[{camera_id}] Redis OBRIGATÓRIO mas está desabilitado no config!")
                self.stats['total_errors'] += 1
                self.stats['redis_misses'] += 1
                # NACK imediato
                if not self.config['rabbitmq']['auto_ack']:
                    ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
                return

            # 3. Busca frame do Redis (SEM FALLBACK para body!)
            frame_data = self._fetch_from_redis(redis_key)
            if frame_data:
                self.stats['redis_hits'] += 1
            else:
                # SEM FALLBACK! Se não está no Redis, é ERRO (TTL expirado ou chave inválida)
                self.stats['redis_misses'] += 1
                logger.error(f"[{camera_id}] Frame NÃO encontrado no Redis (key: {redis_key}) - TTL expirado?")
                self.stats['total_errors'] += 1
                # NACK message
                if not self.config['rabbitmq']['auto_ack']:
                    ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
                return

            # PROCESSA frame (rápido - decodifica + exibe só se for câmera selecionada)
            success = self._process_frame(camera_id, frame_data, headers)

            if success:
                self.stats['total_success'] += 1
                frames_consumed_total.labels(camera_id=camera_id, status='success').inc()

                # ACK message na thread principal (thread-safe!)
                if not self.config['rabbitmq']['auto_ack']:
                    ch.basic_ack(delivery_tag=method.delivery_tag)
            else:
                self.stats['total_errors'] += 1
                frames_consumed_total.labels(camera_id=camera_id, status='error').inc()

                # NACK message na thread principal
                if not self.config['rabbitmq']['auto_ack']:
                    ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

        except Exception as e:
            logger.error(f"[{camera_id}] Callback error: {e}")
            self.stats['total_errors'] += 1
            frames_consumed_total.labels(camera_id=camera_id, status='error').inc()

            # NACK em caso de erro
            if not self.config['rabbitmq']['auto_ack']:
                ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

    def start(self):
        """Start consuming messages"""
        logger.info("=" * 60)
        logger.info("  Edge Video Consumer - Starting")
        logger.info("=" * 60)

        # Start Prometheus metrics server
        if self.config['metrics']['enabled']:
            start_http_server(self.config['metrics']['port'])
            logger.info(f"📊 Metrics server: http://localhost:{self.config['metrics']['port']}/metrics")

        # Connect to Redis
        self._connect_redis()

        # Connect to RabbitMQ
        self._connect_rabbitmq()

        # Reset stats
        self.stats['start_time'] = time.time()
        self.stats['total_consumed'] = 0
        self.stats['total_success'] = 0
        self.stats['total_errors'] = 0
        self.stats['camera_counts'] = {}

        # Start consuming
        logger.info("")
        logger.info("✓ Consumer started successfully!")
        logger.info(f"✓ Consuming from queue: {self.config['rabbitmq']['queue']}")
        logger.info(f"✓ Routing key: {self.config['rabbitmq']['routing_key']}")
        logger.info("✓ Press Ctrl+C to stop")
        logger.info("")

        self.running = True

        try:
            # Setup consumer
            self.channel.basic_consume(
                queue=self.config['rabbitmq']['queue'],
                on_message_callback=self._callback,
                auto_ack=self.config['rabbitmq']['auto_ack']
            )

            # Start consuming (blocking)
            self.channel.start_consuming()

        except KeyboardInterrupt:
            logger.info("Received KeyboardInterrupt, stopping...")
            self.stop()
        except Exception as e:
            logger.error(f"Consumer error: {e}")
            self.stop()
            raise

    def stop(self):
        """Stop consuming and cleanup"""
        if not self.running:
            return

        logger.info("")
        logger.info("Stopping consumer...")
        self.running = False

        # Stop consuming
        if self.channel:
            try:
                self.channel.stop_consuming()
            except:
                pass

        # Close connections
        if self.connection:
            try:
                self.connection.close()
                logger.info("✓ RabbitMQ connection closed")
            except:
                pass

        if self.redis_client:
            try:
                self.redis_client.close()
                logger.info("✓ Redis connection closed")
            except:
                pass

        # Shutdown thread pool
        self.executor.shutdown(wait=True)

        # Fecha todas as janelas OpenCV
        cv2.destroyAllWindows()
        logger.info("✓ OpenCV windows closed")

        # Print final stats
        self._print_final_stats()

        logger.info("✓ Consumer stopped successfully")

    def _print_final_stats(self):
        """Print final statistics"""
        elapsed = time.time() - self.stats['start_time']
        fps = self.stats['total_consumed'] / elapsed if elapsed > 0 else 0

        logger.info("")
        logger.info("=" * 60)
        logger.info("  FINAL STATISTICS")
        logger.info("=" * 60)
        logger.info(f"Uptime:          {elapsed:.1f}s")
        logger.info(f"Total Consumed:  {self.stats['total_consumed']} frames")
        logger.info(f"Success:         {self.stats['total_success']} ({self.stats['total_success']/max(self.stats['total_consumed'],1)*100:.1f}%)")
        logger.info(f"Errors:          {self.stats['total_errors']} ({self.stats['total_errors']/max(self.stats['total_consumed'],1)*100:.1f}%)")
        logger.info(f"Throughput:      {fps:.2f} frames/s")
        logger.info("")
        logger.info("Per-Camera Stats:")
        for camera_id, count in sorted(self.stats['camera_counts'].items()):
            logger.info(f"  [{camera_id}]: {count} frames ({count/elapsed:.1f} fps)")
        logger.info("=" * 60)


# ============================================================================
# MAIN
# ============================================================================
def main():
    """Main entry point"""
    # Check if config exists
    config_path = "config.yaml"
    if not os.path.exists(config_path):
        logger.error(f"Config file not found: {config_path}")
        sys.exit(1)

    # Create and start consumer
    consumer = EdgeVideoConsumer(config_path)
    consumer.start()


if __name__ == "__main__":
    main()
