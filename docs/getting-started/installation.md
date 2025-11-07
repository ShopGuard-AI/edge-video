# Instalação

Este guia mostra como instalar e configurar o Edge Video em diferentes ambientes.

## Pré-requisitos

### Requisitos Mínimos

- **CPU**: 2 cores
- **RAM**: 2 GB
- **Disco**: 10 GB
- **OS**: Linux, macOS ou Windows (com Docker)

### Software Necessário

=== "Docker (Recomendado)"

    - [Docker](https://docs.docker.com/get-docker/) 20.10+
    - [Docker Compose](https://docs.docker.com/compose/install/) 2.0+

=== "Build Local"

    - [Go](https://go.dev/dl/) 1.24+
    - [FFmpeg](https://ffmpeg.org/download.html)
    - [RabbitMQ](https://www.rabbitmq.com/download.html) 3.13+
    - [Redis](https://redis.io/download) 7+

## Instalação via Docker Compose

!!! success "Método Recomendado"
    Esta é a forma mais rápida e fácil de começar!

### 1. Clone o Repositório

```bash
git clone https://github.com/T3-Labs/edge-video.git
cd edge-video
```

### 2. Configure as Câmeras

Edite o arquivo `config.toml`:

```toml
[[cameras]]
id = "cam1"
url = "rtsp://user:pass@192.168.1.100:554/stream"

[[cameras]]
id = "cam2"
url = "rtsp://user:pass@192.168.1.101:554/stream"
```

### 3. Inicie os Serviços

```bash
docker-compose up -d
```

### 4. Verifique os Logs

```bash
# Logs do collector
docker logs -f camera-collector

# Logs do RabbitMQ
docker logs -f rabbitmq

# Logs do Redis
docker logs -f redis
```

### 5. Acesse as Interfaces

| Serviço | URL | Credenciais |
|---------|-----|-------------|
| RabbitMQ Management | http://localhost:15672 | user / password |
| RedisInsight | http://localhost:5540 | - |

## Instalação via Docker Pull

### 1. Pull da Imagem

```bash
docker pull ghcr.io/t3-labs/edge-video:latest
```

### 2. Execute o Container

```bash
docker run -d \
  --name edge-video \
  -v $(pwd)/config.toml:/app/config.toml \
  --network edge-video-network \
  ghcr.io/t3-labs/edge-video:latest
```

## Build e Instalação Local

### 1. Instalar Dependências

=== "Ubuntu/Debian"

    ```bash
    # Go
    wget https://go.dev/dl/go1.24.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.24.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin

    # FFmpeg
    sudo apt update
    sudo apt install -y ffmpeg

    # RabbitMQ
    sudo apt install -y rabbitmq-server
    sudo systemctl start rabbitmq-server

    # Redis
    sudo apt install -y redis-server
    sudo systemctl start redis-server
    ```

=== "macOS"

    ```bash
    # Homebrew
    brew install go
    brew install ffmpeg
    brew install rabbitmq
    brew install redis

    # Iniciar serviços
    brew services start rabbitmq
    brew services start redis
    ```

=== "Windows"

    ```powershell
    # Chocolatey
    choco install golang
    choco install ffmpeg
    
    # RabbitMQ e Redis via Docker
    docker run -d -p 5672:5672 -p 15672:15672 rabbitmq:3.13-management
    docker run -d -p 6379:6379 redis:7-alpine
    ```

### 2. Clone e Build

```bash
# Clone
git clone https://github.com/T3-Labs/edge-video.git
cd edge-video

# Download dependências
go mod download

# Build
go build -o edge-video ./cmd/edge-video

# Executar
./edge-video
```

## Verificação da Instalação

### 1. Verificar Status dos Serviços

```bash
# Docker
docker ps

# Deve mostrar:
# - camera-collector
# - rabbitmq
# - redis
# - redisinsight
```

### 2. Verificar Conectividade

```bash
# RabbitMQ
curl -u user:password http://localhost:15672/api/overview

# Redis
redis-cli -h localhost -p 6379 ping
# Resposta esperada: PONG
```

### 3. Verificar Captura de Frames

```bash
# Ver logs do collector
docker logs camera-collector | grep "frame captured"

# Verificar frames no Redis
redis-cli -h localhost -p 6379 KEYS "frames:*"
```

## Próximos Passos

- [Configuração Detalhada](configuration.md)
- [Quick Start](quickstart.md)
- [Troubleshooting](../guides/troubleshooting.md)

## Desinstalação

### Docker Compose

```bash
# Parar e remover containers
docker-compose down

# Remover volumes (opcional)
docker-compose down -v

# Remover imagens (opcional)
docker rmi edge-video_camera-collector
```

### Local

```bash
# Parar serviços
sudo systemctl stop rabbitmq-server
sudo systemctl stop redis-server

# Remover binário
rm -f edge-video

# Remover dependências (opcional)
sudo apt remove --purge rabbitmq-server redis-server
```
