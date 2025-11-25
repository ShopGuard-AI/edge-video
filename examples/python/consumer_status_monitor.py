#!/usr/bin/env python3
"""
Consumidor de eventos do Edge Video com suporte para eventos de status de câmeras.

Este script demonstra como consumir:
1. Eventos de frames (event_type: "frame")
2. Eventos de status de câmeras (event_type: "camera_status")
3. Eventos de status do sistema (event_type: "system_status")
"""

import json
import pika
import redis
from typing import Optional, Dict, Any
import logging

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class EdgeVideoConsumer:
    """
    Consumidor de eventos do Edge Video com suporte para diferentes tipos de eventos.
    """

    def __init__(
        self,
        rabbitmq_url: str,
        redis_host: str = 'localhost',
        redis_port: int = 6379,
        exchange: str = 'cameras',
        routing_key_pattern: str = 'camera.metadata.#'
    ):
        self.rabbitmq_url = rabbitmq_url
        self.exchange = exchange
        self.routing_key_pattern = routing_key_pattern
        
        self.redis_client = redis.Redis(
            host=redis_host,
            port=redis_port,
            decode_responses=False
        )
        
        self.connection: Optional[pika.BlockingConnection] = None
        self.channel: Optional[pika.adapters.blocking_connection.BlockingChannel] = None
        
        self.frame_count = 0
        self.camera_status_events = 0
        self.system_status_events = 0

    def connect(self):
        """Estabelece conexão com RabbitMQ"""
        try:
            self.connection = pika.BlockingConnection(
                pika.URLParameters(self.rabbitmq_url)
            )
            self.channel = self.connection.channel()
            
            self.channel.exchange_declare(
                exchange=self.exchange,
                exchange_type='topic',
                durable=True
            )
            
            result = self.channel.queue_declare(queue='', exclusive=True)
            queue_name = result.method.queue
            
            self.channel.queue_bind(
                exchange=self.exchange,
                queue=queue_name,
                routing_key=self.routing_key_pattern
            )
            
            logger.info(f"Conectado ao RabbitMQ, aguardando eventos no exchange '{self.exchange}'")
            logger.info(f"Padrão de routing key: {self.routing_key_pattern}")
            
            return queue_name
            
        except Exception as e:
            logger.error(f"Erro ao conectar no RabbitMQ: {e}")
            raise

    def handle_frame_event(self, metadata: Dict[str, Any]):
        """Processa evento de frame"""
        camera_id = metadata.get('camera_id')
        redis_key = metadata.get('redis_key')
        size_bytes = metadata.get('size_bytes', 0)
        
        logger.info(
            f"Frame recebido - Câmera: {camera_id}, "
            f"Tamanho: {size_bytes} bytes"
        )
        
        if redis_key:
            try:
                frame_data = self.redis_client.get(redis_key)
                if frame_data:
                    logger.debug(f"Frame recuperado do Redis: {len(frame_data)} bytes")
                else:
                    logger.warning(f"Frame não encontrado no Redis: {redis_key}")
            except Exception as e:
                logger.error(f"Erro ao buscar frame no Redis: {e}")
        
        self.frame_count += 1

    def handle_camera_status_event(self, event: Dict[str, Any]):
        """Processa evento de status de câmera"""
        camera_id = event.get('camera_id')
        state = event.get('state')
        consecutive_failures = event.get('consecutive_failures', 0)
        last_error = event.get('last_error', '')
        message = event.get('message', '')
        
        log_func = logger.warning if state == 'inactive' else logger.info
        
        log_func(
            f"Status da câmera alterado - "
            f"Câmera: {camera_id}, "
            f"Estado: {state}, "
            f"Falhas consecutivas: {consecutive_failures}, "
            f"Mensagem: {message}"
        )
        
        if last_error:
            logger.error(f"Último erro: {last_error}")
        
        self.camera_status_events += 1
        
        if state == 'inactive':
            self.handle_camera_inactive(camera_id, consecutive_failures, last_error)
        elif state == 'active':
            self.handle_camera_active(camera_id)

    def handle_system_status_event(self, event: Dict[str, Any]):
        """Processa evento de status do sistema"""
        total_cameras = event.get('total_cameras', 0)
        active_cameras = event.get('active_cameras', 0)
        inactive_cameras = event.get('inactive_cameras', 0)
        message = event.get('message', '')
        
        log_func = logger.error if active_cameras == 0 else logger.warning
        
        log_func(
            f"Status do sistema - "
            f"Total de câmeras: {total_cameras}, "
            f"Ativas: {active_cameras}, "
            f"Inativas: {inactive_cameras}, "
            f"Mensagem: {message}"
        )
        
        self.system_status_events += 1
        
        if active_cameras == 0:
            self.handle_no_active_cameras(total_cameras, message)

    def handle_camera_inactive(self, camera_id: str, consecutive_failures: int, last_error: str):
        """
        Handler customizável para quando uma câmera ficar inativa.
        Pode ser sobrescrito para implementar lógica personalizada (alertas, notificações, etc.)
        """
        logger.warning(f"Câmera {camera_id} está inativa após {consecutive_failures} falhas")
        # Adicione aqui sua lógica customizada (ex: enviar alerta, notificação, etc.)

    def handle_camera_active(self, camera_id: str):
        """
        Handler customizável para quando uma câmera voltar a ficar ativa.
        """
        logger.info(f"Câmera {camera_id} voltou a ficar ativa")
        # Adicione aqui sua lógica customizada

    def handle_no_active_cameras(self, total_cameras: int, message: str):
        """
        Handler customizável para quando não houver câmeras ativas.
        """
        logger.critical(
            f"ALERTA CRÍTICO: Nenhuma câmera ativa! "
            f"Total de câmeras: {total_cameras}, Mensagem: {message}"
        )
        # Adicione aqui sua lógica customizada (ex: enviar alerta crítico, página on-call, etc.)

    def on_message(self, ch, method, properties, body):
        """Callback para processar mensagens recebidas"""
        try:
            event = json.loads(body)
            event_type = event.get('event_type', 'frame')
            
            if event_type == 'frame':
                self.handle_frame_event(event)
            elif event_type == 'camera_status':
                self.handle_camera_status_event(event)
            elif event_type == 'system_status':
                self.handle_system_status_event(event)
            else:
                logger.warning(f"Tipo de evento desconhecido: {event_type}")
            
            ch.basic_ack(delivery_tag=method.delivery_tag)
            
        except json.JSONDecodeError as e:
            logger.error(f"Erro ao decodificar JSON: {e}")
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
        except Exception as e:
            logger.error(f"Erro ao processar mensagem: {e}")
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=True)

    def start(self):
        """Inicia o consumidor"""
        try:
            queue_name = self.connect()
            
            self.channel.basic_qos(prefetch_count=10)
            self.channel.basic_consume(
                queue=queue_name,
                on_message_callback=self.on_message
            )
            
            logger.info("Iniciando consumo de eventos...")
            logger.info("Pressione CTRL+C para parar")
            
            self.channel.start_consuming()
            
        except KeyboardInterrupt:
            logger.info("Encerrando consumidor...")
            self.stop()
        except Exception as e:
            logger.error(f"Erro no consumidor: {e}")
            raise

    def stop(self):
        """Encerra o consumidor"""
        if self.channel:
            self.channel.stop_consuming()
        
        if self.connection:
            self.connection.close()
        
        logger.info("Estatísticas finais:")
        logger.info(f"  - Frames processados: {self.frame_count}")
        logger.info(f"  - Eventos de status de câmera: {self.camera_status_events}")
        logger.info(f"  - Eventos de status do sistema: {self.system_status_events}")


def main():
    """Função principal"""
    # Configuração
    RABBITMQ_URL = "amqp://guest:guest@localhost:5672/supermercado_vhost"
    REDIS_HOST = "localhost"
    REDIS_PORT = 6379
    EXCHANGE = "cameras"
    
    # Pode usar diferentes padrões de routing key:
    # - "camera.metadata.#" - todos os eventos de metadata (frames, status, system)
    # - "camera.metadata.event" - apenas eventos de frames
    # - "camera.metadata.status" - apenas eventos de status de câmeras
    # - "camera.metadata.system" - apenas eventos de status do sistema
    ROUTING_KEY_PATTERN = "camera.metadata.#"
    
    consumer = EdgeVideoConsumer(
        rabbitmq_url=RABBITMQ_URL,
        redis_host=REDIS_HOST,
        redis_port=REDIS_PORT,
        exchange=EXCHANGE,
        routing_key_pattern=ROUTING_KEY_PATTERN
    )
    
    consumer.start()


if __name__ == "__main__":
    main()
