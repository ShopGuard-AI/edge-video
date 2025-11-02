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
EXCHANGE_NAME = 'cameras'
ROUTING_KEY = 'camera.#'
QUEUE_NAME = 'test_consumer_queue'

camera_windows = {}

def on_message_received(ch, method, properties, body):
    camera_id = method.routing_key.replace('camera.', '')  # Limpa o prefixo
    print(f" [x] Recebido frame da câmera '{camera_id}'. Tamanho: {len(body)} bytes")

    # Decodifica o frame como imagem (JPEG/PNG) sem descompressão
    try:
        np_arr = np.frombuffer(body, np.uint8)
        img = cv2.imdecode(np_arr, cv2.IMREAD_COLOR)
        if img is not None:
            cv2.imshow(camera_id, img)
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
                # Só verifica a tecla se houver janelas abertas
                if camera_windows and any([cv2.waitKey(1) & 0xFF == ord('q') for _ in camera_windows]):
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