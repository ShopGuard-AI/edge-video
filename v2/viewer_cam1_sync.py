import pika
import sys
import cv2
import numpy as np
from datetime import datetime
from collections import deque
import time

# Configurações do RabbitMQ
RABBITMQ_HOST = '34.71.212.239'
RABBITMQ_PORT = 5672
RABBITMQ_VHOST = 'supercarlao_rj_mercado'
RABBITMQ_USER = 'supercarlao_rj_mercado'
RABBITMQ_PASS = 'vTUaGG9S0Kbl_1AhqPxudML1jCYHRq0efkRDA6p_lgM'

# Escolha qual câmera visualizar
CAMERA_ID = 'cam1'

EXCHANGE_NAME = 'supercarlao_rj_mercado.exchange'
ROUTING_KEY = f'supercarlao_rj_mercado.{CAMERA_ID}'

# Mantém apenas os 3 frames mais recentes
frame_buffer = deque(maxlen=3)
frame_count = 0
last_display_time = 0
fps_target = 15  # 15 FPS = 66.67ms por frame
frame_interval = 1.0 / fps_target

def on_frame_received(ch, method, properties, body):
    """Callback para receber frames"""
    global frame_count, frame_buffer

    frame_count += 1

    try:
        # Decodifica o JPEG
        np_arr = np.frombuffer(body, np.uint8)
        img = cv2.imdecode(np_arr, cv2.IMREAD_COLOR)

        if img is not None:
            # Pega o timestamp do AMQP (se disponível)
            timestamp = properties.timestamp if properties.timestamp else time.time()

            # Adiciona ao buffer com timestamp
            frame_buffer.append({
                'image': img,
                'timestamp': timestamp,
                'count': frame_count,
                'size': len(body)
            })

        else:
            print(f"[ERRO] Não foi possível decodificar frame #{frame_count}")

    except Exception as e:
        print(f"[ERRO] Erro ao processar frame: {e}")

def main():
    global last_display_time

    print("="*80)
    print(f"VISUALIZADOR SINCRONIZADO - {CAMERA_ID}")
    print("="*80)
    print(f"RabbitMQ: {RABBITMQ_HOST}:{RABBITMQ_PORT}")
    print(f"VHost: {RABBITMQ_VHOST}")
    print(f"Exchange: {EXCHANGE_NAME}")
    print(f"Routing Key: {ROUTING_KEY}")
    print(f"Target FPS: {fps_target}")
    print("="*80)
    print("\nPressione 'q' na janela de vídeo ou CTRL+C para sair\n")

    credentials = pika.PlainCredentials(RABBITMQ_USER, RABBITMQ_PASS)
    parameters = pika.ConnectionParameters(
        host=RABBITMQ_HOST,
        port=RABBITMQ_PORT,
        virtual_host=RABBITMQ_VHOST,
        credentials=credentials,
        heartbeat=600,
        blocked_connection_timeout=300
    )

    try:
        connection = pika.BlockingConnection(parameters)
        channel = connection.channel()

        print("[OK] Conectado ao RabbitMQ!\n")

        # Configura consumo com prefetch=1 para não acumular
        channel.basic_qos(prefetch_count=1)
        channel.exchange_declare(exchange=EXCHANGE_NAME, exchange_type='topic', durable=True)
        result = channel.queue_declare(queue='', exclusive=True)
        queue_name = result.method.queue
        channel.queue_bind(exchange=EXCHANGE_NAME, queue=queue_name, routing_key=ROUTING_KEY)
        channel.basic_consume(queue=queue_name, on_message_callback=on_frame_received, auto_ack=True)

        print(f"[OK] Aguardando frames da {CAMERA_ID}...\n")

        last_display_time = time.time()
        frames_displayed = 0
        start_time = time.time()

        # Loop de consumo e display
        while True:
            try:
                # Processa mensagens do RabbitMQ (não bloqueante)
                channel.connection.process_data_events(time_limit=0.01)

                # Controle de FPS - exibe apenas se passou tempo suficiente
                current_time = time.time()
                time_since_last = current_time - last_display_time

                if time_since_last >= frame_interval and len(frame_buffer) > 0:
                    # Pega o frame mais recente do buffer
                    latest_frame = frame_buffer[-1]

                    img = latest_frame['image']

                    # Redimensiona para 50% (640x360)
                    height, width = img.shape[:2]
                    resized = cv2.resize(img, (width // 2, height // 2))

                    # Calcula FPS real
                    elapsed = current_time - start_time
                    fps_real = frames_displayed / elapsed if elapsed > 0 else 0

                    # Adiciona informações
                    cv2.putText(resized, f'{CAMERA_ID}', (5, 20),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 0), 1)
                    cv2.putText(resized, f'Frame #{latest_frame["count"]}', (5, 40),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.4, (0, 255, 0), 1)
                    cv2.putText(resized, f'FPS: {fps_real:.1f}', (5, 60),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.4, (0, 255, 0), 1)
                    cv2.putText(resized, f'Buffer: {len(frame_buffer)}', (5, 80),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.4, (0, 255, 0), 1)
                    cv2.putText(resized, datetime.now().strftime('%H:%M:%S'), (5, 100),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.4, (0, 255, 0), 1)

                    # Exibe o frame
                    cv2.imshow(f'Edge Video - {CAMERA_ID} (Sync)', resized)

                    last_display_time = current_time
                    frames_displayed += 1

                    # Log periódico
                    if frames_displayed % 30 == 0:
                        print(f"[INFO] Exibidos {frames_displayed} frames, FPS real: {fps_real:.1f}, "
                              f"Buffer size: {len(frame_buffer)}, Recebidos: {frame_count}")

                # Processa eventos do OpenCV
                if cv2.waitKey(1) & 0xFF == ord('q'):
                    print("\n[INFO] Saindo...")
                    break

            except KeyboardInterrupt:
                print('\n[INFO] Interrompido pelo usuário')
                break

        cv2.destroyAllWindows()
        connection.close()

        # Estatísticas finais
        total_time = time.time() - start_time
        print(f"\n{'='*80}")
        print(f"ESTATÍSTICAS:")
        print(f"  - Frames recebidos: {frame_count}")
        print(f"  - Frames exibidos: {frames_displayed}")
        print(f"  - Tempo total: {total_time:.1f}s")
        print(f"  - FPS médio: {frames_displayed/total_time:.1f}")
        print(f"  - Taxa de descarte: {((frame_count - frames_displayed)/frame_count*100):.1f}%")
        print(f"{'='*80}")

    except pika.exceptions.AMQPConnectionError as e:
        print(f"\n[ERRO] Erro de conexão: {e}")
    except Exception as e:
        print(f"\n[ERRO] Erro inesperado: {e}")
        import traceback
        traceback.print_exc()

if __name__ == '__main__':
    main()
