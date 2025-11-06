#!/usr/bin/env python3
"""
Test script para consumir metadados de frames do RabbitMQ e buscar frames no Redis.
Este script demonstra a integração completa entre Redis e RabbitMQ para o edge-video.
"""

import json
import sys
import signal
from typing import Optional

import numpy as np
import cv2
import pika
import redis


class CameraFrameConsumer:
    """
    Consome metadados de frames do RabbitMQ e busca os frames correspondentes no Redis.
    """

    def __init__(
        self,
        rabbitmq_url: str = "amqp://user:password@localhost:5672/supermercado_vhost",
        redis_host: str = "localhost",
        redis_port: int = 6379,
        metadata_exchange: str = "camera.metadata",
        metadata_routing_key: str = "camera.metadata.event",
        enable_visualization: bool = True,
    ):
        """
        Inicializa o consumer de frames.

        Args:
            rabbitmq_url: URL de conexão do RabbitMQ
            redis_host: Host do Redis
            redis_port: Porta do Redis
            metadata_exchange: Exchange do RabbitMQ para metadados
            metadata_routing_key: Routing key para metadados
            enable_visualization: Se True, exibe os frames em janelas OpenCV
        """
        self.rabbitmq_url = rabbitmq_url
        self.metadata_exchange = metadata_exchange
        self.metadata_routing_key = metadata_routing_key
        self.enable_visualization = enable_visualization

        # Inicializa conexão com Redis
        self.redis_client = redis.Redis(
            host=redis_host, port=redis_port, decode_responses=False
        )

        # Conexões do RabbitMQ (serão inicializadas no connect)
        self.connection: Optional[pika.BlockingConnection] = None
        self.channel: Optional[pika.channel.Channel] = None
        self.queue_name: Optional[str] = None

        # Estatísticas
        self.messages_received = 0
        self.frames_found = 0
        self.frames_not_found = 0

        # Janelas de visualização (uma por câmera)
        self.windows = {}
        self.window_positions = {
            "cam1": (50, 50),
            "cam2": (700, 50),
            "cam3": (1350, 50),
            "cam4": (50, 500),
            "cam5": (700, 500),
        }

    def connect(self):
        """Estabelece conexão com o RabbitMQ e declara a fila."""
        print(f"Conectando ao RabbitMQ: {self.rabbitmq_url}")

        # Conecta ao RabbitMQ
        parameters = pika.URLParameters(self.rabbitmq_url)
        self.connection = pika.BlockingConnection(parameters)
        self.channel = self.connection.channel()

        # Declara o exchange (caso não exista)
        self.channel.exchange_declare(
            exchange=self.metadata_exchange, exchange_type="topic", durable=True
        )

        # Cria uma fila exclusiva para este consumer
        result = self.channel.queue_declare(queue="", exclusive=True)
        self.queue_name = result.method.queue

        # Faz o bind da fila ao exchange com a routing key
        self.channel.queue_bind(
            exchange=self.metadata_exchange,
            queue=self.queue_name,
            routing_key=self.metadata_routing_key,
        )

        print(f"✅ Conectado ao RabbitMQ")
        print(f"📥 Aguardando mensagens de metadados em '{self.metadata_exchange}'...")
        print(f"🔑 Routing Key: {self.metadata_routing_key}")
        print("-" * 80)

    def get_frame_from_redis(self, redis_key: str) -> Optional[bytes]:
        """
        Busca um frame do Redis pela chave.

        Args:
            redis_key: Chave do frame no Redis

        Returns:
            Dados binários do frame (JPEG) ou None se não encontrado
        """
        try:
            frame_data = self.redis_client.get(redis_key)
            return frame_data
        except redis.RedisError as e:
            print(f"❌ Erro ao buscar frame do Redis: {e}")
            return None

    def decode_frame(self, frame_data: bytes) -> Optional[np.ndarray]:
        """
        Decodifica dados JPEG em uma imagem OpenCV.

        Args:
            frame_data: Dados binários do frame (JPEG)

        Returns:
            Imagem decodificada (numpy array) ou None se houver erro
        """
        try:
            # Converte bytes para numpy array
            nparr = np.frombuffer(frame_data, np.uint8)
            # Decodifica a imagem
            img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
            return img
        except Exception as e:
            print(f"❌ Erro ao decodificar frame: {e}")
            return None

    def display_frame(self, camera_id: str, frame: np.ndarray, metadata: dict):
        """
        Exibe um frame em uma janela OpenCV.

        Args:
            camera_id: ID da câmera
            frame: Imagem (numpy array)
            metadata: Metadados do frame para exibir
        """
        if not self.enable_visualization:
            return

        # Cria uma cópia para adicionar informações
        display_frame = frame.copy()

        # Adiciona informações no frame
        timestamp = metadata.get("timestamp", "N/A")
        width = metadata.get("width", 0)
        height = metadata.get("height", 0)
        size_bytes = metadata.get("size_bytes", 0)

        # Texto de informações
        info_text = [
            f"Camera: {camera_id}",
            f"Time: {timestamp[-15:-5]}",  # Exibe apenas HH:MM:SS
            f"Size: {width}x{height}",
            f"Bytes: {size_bytes:,}",
        ]

        # Adiciona texto no frame
        y_offset = 30
        for i, text in enumerate(info_text):
            cv2.putText(
                display_frame,
                text,
                (10, y_offset + i * 25),
                cv2.FONT_HERSHEY_SIMPLEX,
                0.6,
                (0, 255, 0),
                2,
            )

        # Nome da janela
        window_name = f"Camera {camera_id}"

        # Cria janela se não existir
        if camera_id not in self.windows:
            cv2.namedWindow(window_name, cv2.WINDOW_NORMAL)
            cv2.resizeWindow(window_name, 640, 360)

            # Posiciona a janela
            if camera_id in self.window_positions:
                x, y = self.window_positions[camera_id]
                cv2.moveWindow(window_name, x, y)

            self.windows[camera_id] = window_name

        # Exibe o frame
        cv2.imshow(window_name, display_frame)
        cv2.waitKey(1)  # Necessário para atualizar a janela

    def process_metadata(self, ch, method, properties, body):
        """
        Callback para processar mensagens de metadados do RabbitMQ.

        Args:
            ch: Canal do RabbitMQ
            method: Método da mensagem
            properties: Propriedades da mensagem
            body: Corpo da mensagem (JSON com metadados)
        """
        self.messages_received += 1

        try:
            # Parse do JSON de metadados
            metadata = json.loads(body)

            camera_id = metadata.get("camera_id")
            timestamp = metadata.get("timestamp")
            redis_key = metadata.get("redis_key")
            width = metadata.get("width")
            height = metadata.get("height")
            encoding = metadata.get("encoding")
            size_bytes = metadata.get("size_bytes")

            print(f"\n📸 Frame recebido #{self.messages_received}")
            print(f"   Camera ID: {camera_id}")
            print(f"   Timestamp: {timestamp}")
            print(f"   Redis Key: {redis_key}")
            print(f"   Resolução: {width}x{height}")
            print(f"   Encoding: {encoding}")
            print(f"   Tamanho (metadata): {size_bytes:,} bytes")

            # Busca o frame no Redis
            if redis_key:
                frame_data = self.get_frame_from_redis(redis_key)

                if frame_data:
                    self.frames_found += 1
                    print(f"   ✅ Frame encontrado no Redis: {len(frame_data):,} bytes")

                    # Verifica o TTL restante da chave
                    ttl = self.redis_client.ttl(redis_key)
                    if ttl > 0:
                        print(f"   ⏱️  TTL restante: {ttl} segundos")
                    else:
                        print(f"   ⚠️  TTL: {ttl} (sem expiração ou já expirado)")

                    # Decodifica e exibe o frame
                    if self.enable_visualization:
                        img = self.decode_frame(frame_data)
                        if img is not None:
                            self.display_frame(camera_id, img, metadata)
                            print(f"   🖼️  Frame exibido em janela OpenCV")

                    # Aqui você pode processar o frame
                    # Por exemplo: salvar em disco, processar com OpenCV, etc.
                    # self.save_frame_to_disk(camera_id, timestamp, frame_data)

                else:
                    self.frames_not_found += 1
                    print("   ❌ Frame NÃO encontrado no Redis (pode ter expirado)")

            # Confirma o processamento da mensagem
            ch.basic_ack(delivery_tag=method.delivery_tag)

            # Imprime estatísticas a cada 10 mensagens
            if self.messages_received % 10 == 0:
                self.print_statistics()

        except json.JSONDecodeError as e:
            print(f"❌ Erro ao decodificar JSON: {e}")
            ch.basic_ack(delivery_tag=method.delivery_tag)
        except Exception as e:
            print(f"❌ Erro ao processar mensagem: {e}")
            ch.basic_ack(delivery_tag=method.delivery_tag)

    def print_statistics(self):
        """Imprime estatísticas do consumer."""
        print("\n" + "=" * 80)
        print("📊 ESTATÍSTICAS")
        print(f"   Mensagens recebidas: {self.messages_received}")
        print(f"   Frames encontrados no Redis: {self.frames_found}")
        print(f"   Frames não encontrados: {self.frames_not_found}")
        if self.messages_received > 0:
            success_rate = (self.frames_found / self.messages_received) * 100
            print(f"   Taxa de sucesso: {success_rate:.1f}%")
        print("=" * 80)

    def save_frame_to_disk(self, camera_id: str, timestamp: str, frame_data: bytes):
        """
        Salva um frame em disco (exemplo de processamento).

        Args:
            camera_id: ID da câmera
            timestamp: Timestamp do frame
            frame_data: Dados binários do frame (JPEG)
        """
        import os

        # Cria diretório se não existir
        output_dir = f"frames/{camera_id}"
        os.makedirs(output_dir, exist_ok=True)

        # Limpa o timestamp para usar como nome de arquivo
        clean_timestamp = timestamp.replace(":", "-").replace(".", "-")
        filename = f"{output_dir}/{clean_timestamp}.jpg"

        # Salva o frame
        with open(filename, "wb") as f:
            f.write(frame_data)

        print(f"   💾 Frame salvo em: {filename}")

    def start_consuming(self):
        """Inicia o consumo de mensagens do RabbitMQ."""
        self.connect()

        # Configura o consumer
        self.channel.basic_qos(prefetch_count=1)
        self.channel.basic_consume(
            queue=self.queue_name, on_message_callback=self.process_metadata
        )

        try:
            print("\n🚀 Iniciando consumo de mensagens. Pressione CTRL+C para parar.\n")
            self.channel.start_consuming()
        except KeyboardInterrupt:
            print("\n\n⏹️  Parando consumer...")
            self.stop()

    def stop(self):
        """Para o consumer e fecha as conexões."""
        if self.channel and self.channel.is_open:
            self.channel.stop_consuming()

        if self.connection and self.connection.is_open:
            self.connection.close()

        self.redis_client.close()

        # Fecha todas as janelas OpenCV
        if self.enable_visualization:
            cv2.destroyAllWindows()

        print("\n" + "=" * 80)
        print("📊 ESTATÍSTICAS FINAIS")
        print(f"   Mensagens recebidas: {self.messages_received}")
        print(f"   Frames encontrados no Redis: {self.frames_found}")
        print(f"   Frames não encontrados: {self.frames_not_found}")
        if self.messages_received > 0:
            success_rate = (self.frames_found / self.messages_received) * 100
            print(f"   Taxa de sucesso: {success_rate:.1f}%")
        print("=" * 80)
        print("✅ Consumer encerrado com sucesso!")


def main():
    """Função principal do script."""
    print("=" * 80)
    print("🎥 Camera Frame Consumer - Redis + RabbitMQ + OpenCV")
    print("=" * 80)

    # Configurações (ajuste conforme necessário)
    consumer = CameraFrameConsumer(
        rabbitmq_url="amqp://user:password@localhost:5672/supermercado_vhost",
        redis_host="localhost",
        redis_port=6379,
        metadata_exchange="camera.metadata",
        metadata_routing_key="camera.metadata.event",
        enable_visualization=True,  # Habilita visualização com OpenCV
    )

    print("\n💡 Dica: Para desabilitar a visualização, defina enable_visualization=False")
    print("💡 Pressione 'q' em qualquer janela ou CTRL+C no terminal para sair\n")

    # Configura handler para SIGTERM
    def signal_handler(sig, frame):
        print("\n\n⚠️  Sinal recebido, encerrando...")
        consumer.stop()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Inicia o consumo
    try:
        consumer.start_consuming()
    except Exception as e:
        print(f"❌ Erro fatal: {e}")
        consumer.stop()
        sys.exit(1)


if __name__ == "__main__":
    main()
