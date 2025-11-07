# Configuração

Guia completo de configuração do Edge Video.

## Arquivo de Configuração

O Edge Video usa TOML como formato de configuração. O arquivo padrão é `config.toml`.

## Estrutura Completa

```toml
# FPS desejado para captura
target_fps = 30

# Protocolo: "amqp" ou "mqtt"
protocol = "amqp"

# Configuração AMQP (RabbitMQ)
[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/supermercado_vhost"
exchange = "supermercado_exchange"
routing_key_prefix = "camera."

# Configuração MQTT
[mqtt]
broker = "tcp://localhost:1883"
topic_prefix = "camera/"

# Configuração Redis
[redis]
enabled = true
address = "redis:6379"
password = "your_redis_password"
ttl_seconds = 300
prefix = "frames"

# Configuração de Metadados
[metadata]
enabled = true
exchange = "camera.metadata"
routing_key = "camera.metadata.event"

# Câmeras RTSP
[[cameras]]
id = "cam1"
url = "rtsp://user:pass@ip:port/stream"

[[cameras]]
id = "cam2"
url = "rtsp://user:pass@ip:port/stream"
```

## Parâmetros Detalhados

Veja a documentação completa em desenvolvimento.

[← Instalação](installation.md){ .md-button }
[Quick Start →](quickstart.md){ .md-button .md-button--primary }
