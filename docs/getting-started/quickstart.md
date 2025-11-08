# Quick Start

Este guia vai te ajudar a configurar e executar o **Edge Video** em menos de 5 minutos.

## Pré-requisitos

Antes de começar, certifique-se de ter:

- ✅ [Docker](https://docs.docker.com/get-docker/) instalado (v20.10+)
- ✅ [Docker Compose](https://docs.docker.com/compose/install/) instalado (v2.0+)
- ✅ URLs de câmeras RTSP disponíveis (ou use câmeras de teste)

## Início Rápido (3 Passos)

### 1. Clone o Repositório

=== "HTTPS"
    ```bash
    git clone https://github.com/T3-Labs/edge-video.git
    cd edge-video
    ```

=== "SSH"
    ```bash
    git clone git@github.com:T3-Labs/edge-video.git
    cd edge-video
    ```

### 2. Configure as Câmeras

Edite o arquivo `config.toml` com suas câmeras RTSP:

```toml
# Configuração básica
interval_ms = 500
protocol = "amqp"

[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/guard_vhost"
exchange = "cameras"
routing_key_prefix = "camera"

[redis]
enabled = true
address = "redis:6379"
password = ""
ttl_seconds = 300
prefix = "frames"

# Suas câmeras
[[cameras]]
id = "cam1"
url = "rtsp://usuario:senha@192.168.1.10:554/stream"

[[cameras]]
id = "cam2"
url = "rtsp://usuario:senha@192.168.1.11:554/stream"

[[cameras]]
id = "cam3"
url = "rtsp://usuario:senha@192.168.1.12:554/stream"
```

!!! tip "Câmeras de Teste"
    Se você não tem câmeras RTSP, pode usar streams públicas de teste:
    ```toml
    [[cameras]]
    id = "test1"
    url = "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mp4"
    ```

### 3. Inicie os Serviços

```bash
docker-compose up -d
```

Isso iniciará:

- 🐰 **RabbitMQ** - Message broker (portas 5672, 15672)
- 🔴 **Redis** - Cache de frames (porta 6379)
- 📊 **RedisInsight** - Interface web para Redis (porta 5540)
- 📹 **Camera Collector** - Aplicação principal

## Verificação

### 1. Verifique os Containers

```bash
docker-compose ps
```

Saída esperada:
```
NAME                   STATUS         PORTS
camera-collector       Up 10 seconds  
rabbitmq              Up 15 seconds  0.0.0.0:5672->5672/tcp, 0.0.0.0:15672->15672/tcp
redis                 Up 15 seconds  0.0.0.0:6379->6379/tcp
redis-insight         Up 15 seconds  0.0.0.0:5540->5540/tcp
```

### 2. Verifique os Logs

```bash
# Logs do collector
docker logs -f camera-collector

# Logs do RabbitMQ
docker logs rabbitmq
```

Logs esperados do collector:
```
2025/11/07 19:30:00 Camera collector iniciado
2025/11/07 19:30:00 Conectado ao RabbitMQ
2025/11/07 19:30:00 Conectado ao Redis
2025/11/07 19:30:00 Iniciando captura de 3 câmeras
2025/11/07 19:30:01 [cam1] Frame capturado e publicado
2025/11/07 19:30:01 [cam2] Frame capturado e publicado
2025/11/07 19:30:01 [cam3] Frame capturado e publicado
```

### 3. Acesse as Interfaces Web

| Serviço | URL | Credenciais |
|---------|-----|-------------|
| **RabbitMQ Management** | [http://localhost:15672](http://localhost:15672) | user / password |
| **RedisInsight** | [http://localhost:5540](http://localhost:5540) | - |

## Visualizar Frames

### Consumer Python (Visualização Grid)

O Edge Video inclui um consumer Python que exibe todas as câmeras em uma grade:

```bash
# Instalar dependências
pip install -r requirements.txt

# Ou com UV (recomendado)
uv pip install -r requirements.txt

# Executar consumer
python test_consumer.py
```

Uma janela será aberta mostrando todas as câmeras em tempo real.

**Pressione 'q' para sair.**

### Verificar Frames no Redis

```bash
# Conectar ao Redis
docker exec -it redis redis-cli

# Listar chaves (formato novo v1.2.0+)
KEYS "guard_vhost:frames:*"

# Ver conteúdo de uma chave
GET "guard_vhost:frames:cam1:1731024000123456789:00001"

# Contar frames por câmera
KEYS "guard_vhost:frames:cam1:*" | wc -l

# Verificar TTL
TTL "guard_vhost:frames:cam1:1731024000123456789:00001"
```

### Verificar Mensagens no RabbitMQ

1. Acesse [http://localhost:15672](http://localhost:15672)
2. Login: `user` / `password`
3. Vá para **Queues** e verifique a exchange `cameras`
4. Veja as mensagens sendo publicadas em tempo real

## Testes Práticos

### Teste 1: Publicar Manualmente

```python
# test_publish.py
import pika
import json
import base64

connection = pika.BlockingConnection(
    pika.ConnectionParameters('localhost', 5672, 'guard_vhost',
        pika.PlainCredentials('user', 'password'))
)
channel = connection.channel()

message = {
    "camera_id": "test",
    "timestamp": "2025-11-07T19:30:00Z",
    "data": base64.b64encode(b"fake_jpeg_data").decode()
}

channel.basic_publish(
    exchange='cameras',
    routing_key='camera.test',
    body=json.dumps(message)
)

print("Mensagem publicada!")
connection.close()
```

### Teste 2: Consumir Manualmente

```python
# test_consume.py
import pika

def callback(ch, method, properties, body):
    print(f"Recebido: {body[:100]}...")

connection = pika.BlockingConnection(
    pika.ConnectionParameters('localhost', 5672, 'guard_vhost',
        pika.PlainCredentials('user', 'password'))
)
channel = connection.channel()

channel.exchange_declare(exchange='cameras', exchange_type='topic')
result = channel.queue_declare(queue='', exclusive=True)
queue_name = result.method.queue

channel.queue_bind(exchange='cameras', queue=queue_name, routing_key='camera.#')
channel.basic_consume(queue=queue_name, on_message_callback=callback, auto_ack=True)

print('Aguardando mensagens...')
channel.start_consuming()
```

### Teste 3: Verificar Performance

```bash
# Monitorar taxa de captura
watch -n 1 'docker logs camera-collector 2>&1 | tail -20'

# Monitorar uso de memória Redis
docker exec redis redis-cli INFO memory | grep used_memory_human

# Monitorar mensagens RabbitMQ
docker exec rabbitmq rabbitmqctl list_queues
```

## Configurações Avançadas

### Multi-Tenant (Múltiplos Clientes)

Para isolar dados de diferentes clientes, use vhosts diferentes:

```toml
# config-client-a.toml
[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/client-a"

# config-client-b.toml
[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/client-b"
```

Execute múltiplas instâncias:

```bash
# Cliente A
docker run -d --name collector-client-a \
  --network edge-video_default \
  -v $(pwd)/config-client-a.toml:/app/config.toml \
  t3labs/edge-video:latest

# Cliente B
docker run -d --name collector-client-b \
  --network edge-video_default \
  -v $(pwd)/config-client-b.toml:/app/config.toml \
  t3labs/edge-video:latest
```

**Resultado no Redis:**
```redis
client-a:frames:cam1:1731024000123456789:00001
client-b:frames:cam1:1731024000123456789:00001
```

### Ajustar Performance

```toml
# Intervalo de captura (menor = mais frames)
interval_ms = 100  # 10 FPS

# Processar a cada N frames (maior = menos carga)
process_every_n_frames = 5  # Captura 1 a cada 5

# TTL do Redis (menor = menos memória)
[redis]
ttl_seconds = 60  # 1 minuto
```

### Habilitar Compressão

```toml
[compression]
enabled = true
level = 5  # 1-22 (maior = mais compressão, mais CPU)
```

### Autenticação Redis

```toml
[redis]
password = "sua_senha_segura"
```

```yaml
# docker-compose.yml
services:
  redis:
    command: redis-server --requirepass sua_senha_segura
```

## Troubleshooting Rápido

### Problema: Containers não iniciam

```bash
# Ver logs
docker-compose logs

# Reiniciar
docker-compose down
docker-compose up -d
```

### Problema: Erro de conexão RTSP

```bash
# Testar URL manualmente com FFmpeg
docker run --rm -it linuxserver/ffmpeg \
  -rtsp_transport tcp \
  -i "rtsp://url_da_camera" \
  -frames:v 1 \
  -f image2 \
  output.jpg
```

### Problema: Redis cheio

```bash
# Verificar uso
docker exec redis redis-cli INFO memory

# Limpar cache (cuidado!)
docker exec redis redis-cli FLUSHDB

# Reduzir TTL no config.toml
[redis]
ttl_seconds = 60  # Menor valor
```

### Problema: RabbitMQ lento

```bash
# Verificar filas
docker exec rabbitmq rabbitmqctl list_queues

# Limpar filas (cuidado!)
docker exec rabbitmq rabbitmqctl purge_queue nome_da_fila
```

## Próximos Passos

Agora que você tem o Edge Video rodando, explore:

<div class="grid cards" markdown>

-   :material-cog:{ .lg } __Configuração Detalhada__
    
    Aprenda todas as opções de configuração
    
    [:octicons-arrow-right-24: Ver configuração](configuration.md)

-   :material-domain:{ .lg } __Multi-Tenancy__
    
    Configure isolamento por cliente
    
    [:octicons-arrow-right-24: Vhost Guide](../vhost-based-identification.md)

-   :material-memory:{ .lg } __Redis Storage__
    
    Otimize cache e performance
    
    [:octicons-arrow-right-24: Redis Guide](../features/redis-storage.md)

-   :material-chart-line:{ .lg } __Monitoramento__
    
    Configure métricas e alertas
    
    [:octicons-arrow-right-24: Monitoring](../guides/monitoring.md)

-   :material-docker:{ .lg } __Deploy Produção__
    
    Prepare para ambiente produtivo
    
    [:octicons-arrow-right-24: Docker Guide](../guides/docker.md)

-   :material-bug:{ .lg } __Troubleshooting__
    
    Resolva problemas comuns
    
    [:octicons-arrow-right-24: Troubleshooting](../guides/troubleshooting.md)

</div>

## Comandos Úteis

### Docker Compose

```bash
# Iniciar serviços
docker-compose up -d

# Ver logs
docker-compose logs -f

# Parar serviços
docker-compose stop

# Remover tudo
docker-compose down -v

# Rebuildar imagens
docker-compose up -d --build

# Escalar collectors
docker-compose up -d --scale camera-collector=3
```

### Container Específico

```bash
# Logs de um serviço
docker logs -f camera-collector

# Entrar no container
docker exec -it camera-collector sh

# Reiniciar serviço
docker-compose restart camera-collector

# Ver recursos
docker stats camera-collector
```

### Redis

```bash
# CLI interativo
docker exec -it redis redis-cli

# Comandos rápidos
docker exec redis redis-cli KEYS "*"
docker exec redis redis-cli DBSIZE
docker exec redis redis-cli INFO memory
docker exec redis redis-cli FLUSHDB
```

### RabbitMQ

```bash
# Status
docker exec rabbitmq rabbitmqctl status

# Listar vhosts
docker exec rabbitmq rabbitmqctl list_vhosts

# Listar usuários
docker exec rabbitmq rabbitmqctl list_users

# Listar filas
docker exec rabbitmq rabbitmqctl list_queues
```

## Dúvidas?

- 📖 [Documentação Completa](../index.md)
- 💬 [Abrir Issue](https://github.com/T3-Labs/edge-video/issues)
- 🐛 [Reportar Bug](https://github.com/T3-Labs/edge-video/issues/new?template=bug_report.md)
- ✨ [Solicitar Feature](https://github.com/T3-Labs/edge-video/issues/new?template=feature_request.md)

