#!/usr/bin/env python3
"""
Consumer de teste para validar funcionamento do Edge Video V2
Conecta no RabbitMQ, pega redis_key, busca frame no Redis e valida
"""

import pika
import redis
import json
import sys
import time
from datetime import datetime

# ============================================================================
# CONFIGURAÇÃO (mesmas credenciais do config.yaml)
# ============================================================================
RABBITMQ_URL = "amqp://supercarlao_rj_mercado:vTUaGG9S0Kbl_1AhqPxudML1jCYHRq0efkRDA6p_lgM@34.71.212.239:5672/supercarlao_rj_mercado"
EXCHANGE = "supercarlao_rj_mercado.exchange"
QUEUE_NAME = "test_consumer_queue"  # Queue temporária para teste
ROUTING_KEY = "supercarlao_rj_mercado.#"  # Escuta TODAS as câmeras

REDIS_HOST = "34.30.59.236"
REDIS_PORT = 6379
REDIS_PASSWORD = "MinhaSenhaMuitoForte@2024"
REDIS_DB = 0

# ============================================================================
# Conecta no Redis
# ============================================================================
print("🔌 Conectando ao Redis...")
redis_client = redis.Redis(
    host=REDIS_HOST,
    port=REDIS_PORT,
    password=REDIS_PASSWORD,
    db=REDIS_DB,
    decode_responses=False  # Retorna bytes (frames JPEG)
)

try:
    redis_client.ping()
    print("✅ Redis conectado!")
except Exception as e:
    print(f"❌ Erro ao conectar no Redis: {e}")
    sys.exit(1)

# ============================================================================
# Conecta no RabbitMQ
# ============================================================================
print(f"🔌 Conectando ao RabbitMQ: {EXCHANGE}...")
params = pika.URLParameters(RABBITMQ_URL)
connection = pika.BlockingConnection(params)
channel = connection.channel()

# Declara exchange (se não existir)
channel.exchange_declare(
    exchange=EXCHANGE,
    exchange_type='topic',
    durable=True
)

# Cria queue temporária exclusiva com timestamp (garante queue nova sempre)
queue_name_unique = f"{QUEUE_NAME}_{int(time.time())}"
result = channel.queue_declare(queue=queue_name_unique, exclusive=True, auto_delete=True)
queue_name = result.method.queue

# Bind na queue para escutar TODAS as câmeras
channel.queue_bind(
    exchange=EXCHANGE,
    queue=queue_name,
    routing_key=ROUTING_KEY
)

print(f"✅ RabbitMQ conectado!")
print(f"📥 Escutando exchange '{EXCHANGE}' com routing_key '{ROUTING_KEY}'")
print(f"🔊 Queue: {queue_name}")
print("-" * 80)
print("✨ Queue NOVA criada - Mensagens antigas IGNORADAS!")
print("⏰ Apenas frames NOVOS (dentro do TTL de 120s) serão recebidos")
print("-" * 80)
print("⏳ Aguardando mensagens... (Ctrl+C para parar)")
print("=" * 80)

# ============================================================================
# Callback para processar mensagens
# ============================================================================
frame_count = 0

def callback(ch, method, properties, body):
    global frame_count
    frame_count += 1

    timestamp = datetime.now().strftime("%H:%M:%S")
    camera_id = properties.headers.get('camera_id', 'UNKNOWN') if properties.headers else 'UNKNOWN'

    try:
        # V1.6 Style: body contém apenas redis_key (texto)
        redis_key = body.decode('utf-8')

        print(f"\n[{timestamp}] 📩 Frame #{frame_count} recebido")
        print(f"  📹 Camera: {camera_id}")
        print(f"  🔑 Redis Key: {redis_key}")
        print(f"  📊 Body Size: {len(body)} bytes (deve ser ~50-100 bytes se for redis_key)")

        # Valida se parece com redis_key
        if len(body) > 500:
            print(f"  ⚠️  WARNING: Body muito grande ({len(body)} bytes)!")
            print(f"  ⚠️  Parece que está recebendo FRAME COMPLETO ao invés de redis_key!")
            print(f"  ⚠️  Body preview: {body[:100]}...")
        else:
            # Busca frame no Redis
            print(f"  🔍 Buscando frame no Redis...")
            frame_data = redis_client.get(redis_key)

            if frame_data:
                frame_size = len(frame_data)
                print(f"  ✅ Frame encontrado no Redis! Size: {frame_size:,} bytes ({frame_size/1024:.1f} KB)")

                # Valida se é JPEG
                if frame_data[:2] == b'\xff\xd8':
                    print(f"  ✅ Frame é JPEG válido (começa com FFD8)")
                else:
                    print(f"  ❌ Frame NÃO é JPEG válido! Header: {frame_data[:10].hex()}")
            else:
                print(f"  ❌ Frame NÃO encontrado no Redis! (pode ter expirado - TTL: 120s)")

        # ACK da mensagem
        ch.basic_ack(delivery_tag=method.delivery_tag)

    except UnicodeDecodeError:
        print(f"  ❌ ERRO: Body NÃO é UTF-8 (parece ser frame binário completo!)")
        print(f"  ❌ Body preview (hex): {body[:50].hex()}")
        ch.basic_ack(delivery_tag=method.delivery_tag)
    except Exception as e:
        print(f"  ❌ ERRO ao processar: {e}")
        ch.basic_ack(delivery_tag=method.delivery_tag)

# ============================================================================
# Inicia consumo
# ============================================================================
channel.basic_qos(prefetch_count=1)
channel.basic_consume(queue=queue_name, on_message_callback=callback)

try:
    channel.start_consuming()
except KeyboardInterrupt:
    print("\n\n🛑 Parando consumer...")
    channel.stop_consuming()
    connection.close()
    print("✅ Consumer parado!")
    print(f"📊 Total de frames processados: {frame_count}")
