# Metadata Publisher

O **Metadata Publisher** envia eventos de metadados para o RabbitMQ sempre que um frame é capturado, permitindo que aplicações consumidoras sejam notificadas em tempo real sem precisar processar o frame completo.

## Visão Geral

O Metadata Publisher oferece:

- ✅ **Eventos em Tempo Real**: Notificação instantânea de novos frames
- ✅ **Payload Leve**: JSON compacto (~200 bytes vs MB de frame)
- ✅ **Multi-Tenant**: Isolamento por vhost do RabbitMQ
- ✅ **Referência Redis**: Chave exata para buscar frame do cache
- ✅ **Desacoplamento**: Consumers decidem se/quando buscar frame completo

## Arquitetura

```mermaid
graph LR
    A[Câmera] -->|Frame| B[Camera Collector]
    B -->|Frame Data| C[Redis]
    B -->|Metadata JSON| D[RabbitMQ]
    D -->|Event| E[Consumer 1]
    D -->|Event| F[Consumer 2]
    E -.->|Fetch| C
    F -.->|Fetch| C
```

**Fluxo:**
1. Camera Collector captura frame da câmera
2. Frame armazenado no Redis com chave única
3. Metadata enviado ao RabbitMQ com referência à chave
4. Consumers recebem notificação leve
5. Consumers buscam frame do Redis apenas se necessário

## Configuração Básica

=== "config.toml"

    ```toml
    [metadata]
    enabled = true
    exchange = "camera.metadata"
    routing_key = "camera.metadata.event"
    
    [amqp]
    amqp_url = "amqp://user:pass@rabbitmq:5672/meu-cliente"
    
    [redis]
    enabled = true
    prefix = "frames"
    ttl_seconds = 300
    ```

=== "Docker Compose"

    ```yaml
    services:
      camera-collector:
        image: t3labs/edge-video:latest
        environment:
          - METADATA_ENABLED=true
          - METADATA_EXCHANGE=camera.metadata
          - REDIS_ENABLED=true
    ```

## Formato do Metadata

### Estrutura JSON

```json
{
  "camera_id": "cam4",
  "timestamp": "2024-11-08T14:30:00.123456789Z",
  "timestamp_nano": 1731073800123456789,
  "sequence": "00001",
  "redis_key": "meu-cliente:frames:cam4:1731073800123456789:00001",
  "vhost": "meu-cliente",
  "frame_size_bytes": 245678,
  "ttl_seconds": 300
}
```

### Campos

| Campo | Tipo | Descrição | Exemplo |
|-------|------|-----------|---------|
| `camera_id` | `string` | Identificador da câmera | `"cam4"` |
| `timestamp` | `string` | Timestamp RFC3339 (legibilidade) | `"2024-11-08T14:30:00.123456789Z"` |
| `timestamp_nano` | `int64` | Unix nanoseconds (performance) | `1731073800123456789` |
| `sequence` | `string` | Sequência anti-colisão (5 dígitos) | `"00001"` |
| `redis_key` | `string` | Chave completa no Redis | `"meu-cliente:frames:cam4:1731073800123456789:00001"` |
| `vhost` | `string` | Vhost do RabbitMQ | `"meu-cliente"` |
| `frame_size_bytes` | `int` | Tamanho do frame em bytes | `245678` |
| `ttl_seconds` | `int` | TTL do frame no Redis | `300` |

EOFMARKER! tip "Unix Nanoseconds"
    Use `timestamp_nano` para comparações e ordenação. Use `timestamp` apenas para exibição humana.

## Consumindo Metadados

### Python Consumer Básico

```python
import pika
import json
import redis

# Conexão RabbitMQ
credentials = pika.PlainCredentials('user', 'password')
parameters = pika.ConnectionParameters(
    host='rabbitmq',
    virtual_host='meu-cliente',
    credentials=credentials
)
connection = pika.BlockingConnection(parameters)
channel = connection.channel()

# Conexão Redis
r = redis.Redis(host='redis', port=6379, decode_responses=False)

def callback(ch, method, properties, body):
    """Processa metadata recebido."""
    metadata = json.loads(body)
    
    print(f"Novo frame: {metadata['camera_id']}")
    print(f"Timestamp: {metadata['timestamp']}")
    print(f"Tamanho: {metadata['frame_size_bytes']} bytes")
    
    # Decidir se busca frame completo
    if metadata['camera_id'] == 'cam4':
        redis_key = metadata['redis_key']
        frame_data = r.get(redis_key)
        
        if frame_data:
            print(f"Frame obtido: {len(frame_data)} bytes")
            # Processar frame...
        else:
            print("Frame expirou no Redis")
    
    ch.basic_ack(delivery_tag=method.delivery_tag)

# Configurar exchange e queue
channel.exchange_declare(exchange='camera.metadata', exchange_type='topic')
result = channel.queue_declare(queue='', exclusive=True)
queue_name = result.method.queue

channel.queue_bind(
    exchange='camera.metadata',
    queue=queue_name,
    routing_key='camera.metadata.event'
)

channel.basic_consume(
    queue=queue_name,
    on_message_callback=callback
)

print('Aguardando metadados...')
channel.start_consuming()
```

## Próximos Passos

<div class="grid cards" markdown>

-   :material-database:{ .lg } __Redis Storage__
    
    Entenda o formato de chaves e operações
    
    [:octicons-arrow-right-24: Redis Guide](redis-storage.md)

-   :material-cube-outline:{ .lg } __Multi-Tenancy__
    
    Isolamento por vhost do RabbitMQ
    
    [:octicons-arrow-right-24: Vhost Guide](../vhost-based-identification.md)

-   :material-cog:{ .lg } __Configuração__
    
    Parâmetros e exemplos completos
    
    [:octicons-arrow-right-24: Configuration](../getting-started/configuration.md)

</div>
