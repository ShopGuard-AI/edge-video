# Edge Video

> Sistema distribuído de captura e processamento de vídeo RTSP para edge computing

[![Go Tests](https://github.com/T3-Labs/edge-video/actions/workflows/go-test.yml/badge.svg)](https://github.com/T3-Labs/edge-video/actions/workflows/go-test.yml)
[![Docker Build](https://github.com/T3-Labs/edge-video/actions/workflows/build-and-push.yml/badge.svg)](https://github.com/T3-Labs/edge-video/actions/workflows/build-and-push.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## 🎯 Sobre

**Edge Video** é uma plataforma robusta para captura, armazenamento e distribuição de vídeo de câmeras RTSP/IP, projetada para ambientes de edge computing com suporte multi-tenant e integração nativa com sistemas de IA.

### Principais Features

- 🎥 **Multi-Câmera RTSP/IP** - Captura simultânea de múltiplas câmeras
- 🏢 **Multi-Tenant (RabbitMQ vhost)** - Isolamento completo de dados por cliente
- 💾 **Redis Otimizado** - Chaves Unix nanoseconds, queries temporais eficientes
- 🚀 **Distribuição AMQP/MQTT** - Flexibilidade para diferentes integrações
- 🧠 **Controle de Memória** - Previne travamento do SO em ambientes restritos
- ⚡ **Worker Pool + Circuit Breaker** - Alta performance e resiliência
- 📊 **Métricas Prometheus** - Observabilidade completa
- 🪟 **Instalador Windows** - Deploy como serviço Windows nativo
- 🐳 **Docker/Docker Compose** - Deploy containerizado pronto para produção

---

## 🚀 Quick Start

### Opção 1: Executável Local

```bash
# 1. Clone o repositório
git clone https://github.com/T3-Labs/edge-video.git
cd edge-video

# 2. Copie a configuração de exemplo
cp configs/config.example.toml config.toml

# 3. Edite config.toml com suas câmeras e credenciais
nano config.toml

# 4. Compile e execute
go build -o edge-video ./cmd/edge-video
./edge-video --config config.toml
```

### Opção 2: Docker Compose (Recomendado)

```bash
# 1. Clone o repositório
git clone https://github.com/T3-Labs/edge-video.git
cd edge-video

# 2. Configure suas câmeras
cp configs/config.example.toml config.toml
nano config.toml

# 3. Inicie os serviços
cd configs/docker-compose
docker-compose up -d

# 4. Monitore os logs
docker-compose logs -f camera-collector
```

### Opção 3: Windows Service

1. Baixe o instalador no [GitHub Releases](https://github.com/T3-Labs/edge-video/releases)
2. Execute o instalador como Administrador
3. Configure em `C:\Program Files\T3Labs\EdgeVideo\config\config.toml`
4. Gerencie via `Services.msc` ou comandos:
   ```cmd
   net start EdgeVideoService
   net stop EdgeVideoService
   ```

---

## 📋 Exemplo de Configuração

```toml
target_fps = 2.0
protocol = "amqp"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/meu-cliente"
exchange = "cameras"
routing_key_prefix = "camera."

[redis]
enabled = true
address = "redis:6379"
ttl_seconds = 300

[memory]
enabled = true
max_memory_mb = 1024
warning_percent = 60.0

[[cameras]]
id = "cam1"
name = "Câmera Entrada"
url = "rtsp://admin:pass@192.168.1.100:554/stream1"
```

📚 Ver [configuração completa](configs/config.example.toml)

---

## 🐍 Consumer Python

```python
import pika
import redis
import json

# Conectar Redis
redis_client = redis.Redis(host='localhost', port=6379)

# Conectar RabbitMQ
connection = pika.BlockingConnection(
    pika.URLParameters('amqp://user:pass@localhost:5672/meu-cliente')
)
channel = connection.channel()

def callback(ch, method, properties, body):
    metadata = json.loads(body)
    
    # Buscar frame do Redis
    frame_bytes = redis_client.get(metadata['redis_key'])
    
    if frame_bytes:
        # Processar frame (OpenCV, IA, etc)
        print(f"Frame recebido: {metadata['camera_id']} - {len(frame_bytes)} bytes")

channel.basic_consume(queue='camera_frames', on_message_callback=callback)
channel.start_consuming()
```

📚 Ver [exemplos completos](examples/python/)

---

## 📊 Monitoramento

### RabbitMQ Management
- URL: `http://localhost:15672`
- Usuário: `user` / Senha: `password`

### Métricas Prometheus
- URL: `http://localhost:9090/metrics`
- Métricas disponíveis:
  - `edge_video_frames_processed_total`
  - `edge_video_memory_usage_percent`
  - `edge_video_camera_connected`
  - `edge_video_buffer_size`

### Logs
- **Linux/macOS**: `logs/edge-video.log`
- **Windows**: Event Viewer → Application → EdgeVideoService
- **Docker**: `docker logs camera-collector`

---

## 📁 Estrutura do Projeto

```
edge-video/
├── cmd/                    # Aplicações executáveis
│   ├── edge-video/        # Aplicação principal
│   └── edge-video-service/# Windows service wrapper
├── pkg/                    # Pacotes reutilizáveis
│   ├── camera/            # Captura RTSP
│   ├── memcontrol/        # Controle de memória
│   ├── mq/                # Publishers AMQP/MQTT
│   └── ...
├── internal/              # Código interno privado
├── configs/               # Arquivos de configuração
│   ├── config.example.toml
│   ├── config.memory-control.toml
│   └── docker-compose/
├── examples/              # Exemplos de uso
│   ├── python/           # Consumers Python
│   └── go/               # Utilitários Go
├── docs/                  # Documentação completa
├── scripts/              # Scripts de build/deploy
└── installer/            # Instalador Windows
```

---

## 📚 Documentação

### Getting Started
- [Instalação](docs/getting-started/installation.md)
- [Configuração](docs/getting-started/configuration.md)
- [Quick Reference](docs/QUICK_REFERENCE.md)

### Features
- [Captura de Câmeras](docs/features/camera-capture.md)
- [Controle de Memória](docs/MEMORY-CONTROL.md)
- [Armazenamento Redis](docs/features/redis-storage.md)
- [Message Queue](docs/features/message-queue.md)
- [Multi-tenancy](docs/features/multi-tenancy.md)

### Guides
- [Implementação Multi-Tenant](docs/guides/vhost-implementation.md)
- [Deploy no Windows](docs/windows/README.md)
- [Integração Python](examples/python/README.md)

### Development
- [Contribuindo](CONTRIBUTING.md)
- [Arquitetura](docs/architecture/overview.md)
- [API Reference](docs/api/)
- [Testing](docs/development/testing.md)

📖 **Documentação completa**: [https://t3-labs.github.io/edge-video/](https://t3-labs.github.io/edge-video/)

---

## 🛠️ Desenvolvimento

### Requisitos
- Go 1.24+
- Docker & Docker Compose (opcional)
- FFmpeg (para captura RTSP)

### Build Local

```bash
# Compilar
go build -o edge-video ./cmd/edge-video

# Executar testes
go test ./...

# Executar com race detector
go test -race ./...

# Gerar coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Build para Windows

```bash
# Linux/macOS → Windows
./scripts/build-windows.sh

# Gerar instalador (requer NSIS)
cd installer/windows
makensis edge-video-installer.nsi
```

### Docker

```bash
# Build local
docker build -t edge-video:latest .

# Build e push
docker buildx build --platform linux/amd64,linux/arm64 -t edge-video:latest --push .
```

---

## 🧪 Testes

```bash
# Testes unitários
go test ./...

# Testes com verbose
go test -v ./...

# Testes de integração
go test -tags=integration ./...

# Benchmarks
go test -bench=. ./...

# Coverage
go test -coverprofile=coverage.out ./...
```

---

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor, leia nosso [Guia de Contribuição](CONTRIBUTING.md).

### Processo

1. Fork o projeto
2. Crie uma branch: `git checkout -b feature/nova-funcionalidade`
3. Commit suas mudanças: `git commit -m 'feat: adiciona nova funcionalidade'`
4. Push para a branch: `git push origin feature/nova-funcionalidade`
5. Abra um Pull Request

### Convenções

- Commits seguem [Conventional Commits](https://www.conventionalcommits.org/)
- Código segue `golangci-lint` rules
- Testes obrigatórios para novas features
- Documentação atualizada

---

## 📄 Licença

Este projeto está licenciado sob a [MIT License](LICENSE).

---

## 🙏 Créditos

Desenvolvido por [T3 Labs](https://github.com/T3-Labs)

### Tecnologias

- [Go](https://go.dev/) - Linguagem principal
- [FFmpeg](https://ffmpeg.org/) - Captura RTSP
- [RabbitMQ](https://www.rabbitmq.com/) - Message broker
- [Redis](https://redis.io/) - Cache e storage
- [Docker](https://www.docker.com/) - Containerização
- [MkDocs](https://www.mkdocs.org/) - Documentação

---

## 📞 Suporte

- 📧 Email: [suporte@t3labs.com](mailto:suporte@t3labs.com)
- 💬 Issues: [GitHub Issues](https://github.com/T3-Labs/edge-video/issues)
- 📚 Docs: [https://t3-labs.github.io/edge-video/](https://t3-labs.github.io/edge-video/)

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/T3-Labs">T3 Labs</a>
</p>
