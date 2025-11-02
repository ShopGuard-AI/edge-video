import pika
import sys
import os
import cv2
import numpy as np

RABBITMQ_HOST = 'localhost'
RABBITMQ_PORT = 5672
RABBITMQ_VHOST = 'guard_vhost'
RABBITMQ_USER = 'user'
RABBITMQ_PASS = 'password'
EXCHANGE_NAME = 'carnes_nobres'
ROUTING_KEY = 'camera.#'
QUEUE_NAME = 'test_consumer_queue'

camera_windows = {}
camera_frames = {}  # Armazena os últimos frames de cada câmera

def on_message_received(ch, method, properties, body):
    camera_id = method.routing_key.replace('camera.', '')  # Limpa o prefixo
    print(f" [x] Recebido frame da câmera '{camera_id}'. Tamanho: {len(body)} bytes")

    # Decodifica o frame como imagem (JPEG/PNG) sem descompressão
    try:
        np_arr = np.frombuffer(body, np.uint8)
        img = cv2.imdecode(np_arr, cv2.IMREAD_COLOR)
        if img is not None:
            camera_frames[camera_id] = img
            camera_windows[camera_id] = True
        else:
            print(f" [!] Não foi possível decodificar o frame da câmera '{camera_id}'.")
    except Exception as e:
        print(f" [!] Erro ao decodificar imagem da câmera '{camera_id}': {e}")

def main():
    credentials = pika.PlainCredentials(RABBITMQ_USER, RABBITMQ_PASS)
    parameters = pika.ConnectionParameters(
        host=RABBITMQ_HOST,
        port=RABBITMQ_PORT,
        virtual_host=RABBITMQ_VHOST,
        credentials=credentials
    )

    try:
        connection = pika.BlockingConnection(parameters)
        channel = connection.channel()
        channel.exchange_declare(exchange=EXCHANGE_NAME, exchange_type='topic', durable=True)
        result = channel.queue_declare(queue=QUEUE_NAME, exclusive=True, durable=False)
        queue_name = result.method.queue
        print(f"[*] Fila '{queue_name}' criada. Aguardando por frames...")
        channel.queue_bind(exchange=EXCHANGE_NAME, queue=queue_name, routing_key=ROUTING_KEY)
        channel.basic_consume(queue=queue_name, on_message_callback=on_message_received, auto_ack=True)

        print("[INFO] Pressione 'q' em qualquer janela para sair.")
        while True:
            try:
                channel.connection.process_data_events(time_limit=0.1)
                
                # Concatena os frames em uma grade 2x3 (6 câmeras)
                if len(camera_frames) > 0:
                    # Cria uma lista ordenada de frames
                    cam_ids = sorted(camera_frames.keys())
                    frames_list = []
                    
                    for cam_id in cam_ids[:6]:  # Limita a 6 câmeras
                        frame = camera_frames[cam_id]
                        # Redimensiona para 640x480 para uniformidade
                        resized = cv2.resize(frame, (640, 480))
                        # Adiciona texto com o ID da câmera
                        cv2.putText(resized, cam_id, (10, 30), cv2.FONT_HERSHEY_SIMPLEX, 
                                    1, (0, 255, 0), 2, cv2.LINE_AA)
                        frames_list.append(resized)
                    
                    # Preenche com frames pretos se houver menos de 6 câmeras
                    while len(frames_list) < 6:
                        black_frame = np.zeros((480, 640, 3), dtype=np.uint8)
                        cv2.putText(black_frame, f"Cam{len(frames_list)+1}", (250, 240), 
                                    cv2.FONT_HERSHEY_SIMPLEX, 1, (255, 255, 255), 2, cv2.LINE_AA)
                        frames_list.append(black_frame)
                    
                    # Concatena em grade 2x3
                    row1 = np.hstack(frames_list[0:3])
                    row2 = np.hstack(frames_list[3:6])
                    grid = np.vstack([row1, row2])
                    
                    cv2.imshow('Cameras Grid (2x3)', grid)
                
                # Verifica se a tecla 'q' foi pressionada
                if cv2.waitKey(1) & 0xFF == ord('q'):
                    print("Saindo por comando do usuário.")
                    break
            except KeyboardInterrupt:
                print('Interrompido pelo usuário. Encerrando.')
                break
        cv2.destroyAllWindows()

    except pika.exceptions.AMQPConnectionError as e:
        print(f"Erro de conexão com o RabbitMQ: {e}")
        print("Verifique se o host, porta, vhost e credenciais estão corretos e se o contêiner do RabbitMQ está em execução.")
    except KeyboardInterrupt:
        print('Interrompido pelo usuário. Encerrando.')
        try:
            sys.exit(0)
        except SystemExit:
            os._exit(0)

if __name__ == '__main__':
    main()