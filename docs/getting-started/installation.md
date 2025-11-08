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

Se você prefere usar a imagem oficial publicada, siga estes passos:

### 1. Pull da Imagem

```bash
# Última versão
docker pull ghcr.io/t3-labs/edge-video:latest

# Versão específica
docker pull ghcr.io/t3-labs/edge-video:v1.2.0
```

### 2. Preparar Configuração

Crie um arquivo `config.toml` local:

```toml
interval_ms = 500
protocol = "amqp"

[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/meu_cliente"
exchange = "cameras"
routing_key_prefix = "camera"

[redis]
enabled = true
address = "redis:6379"
ttl_seconds = 300
prefix = "frames"

[[cameras]]
id = "cam1"
url = "rtsp://usuario:senha@192.168.1.10:554/stream"
```

### 3. Criar Rede Docker

```bash
# Criar rede dedicada
docker network create edge-video-network
```

### 4. Iniciar Serviços de Infraestrutura

```bash
# RabbitMQ
docker run -d \
  --name rabbitmq \
  --network edge-video-network \
  -p 5672:5672 \
  -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=user \
  -e RABBITMQ_DEFAULT_PASS=password \
  rabbitmq:3.13-management-alpine

# Redis
docker run -d \
  --name redis \
  --network edge-video-network \
  -p 6379:6379 \
  redis:7-alpine

# RedisInsight (opcional)
docker run -d \
  --name redisinsight \
  --network edge-video-network \
  -p 5540:5540 \
  redis/redisinsight:latest
```

### 5. Execute o Edge Video

```bash
docker run -d \
  --name edge-video \
  --network edge-video-network \
  -v $(pwd)/config.toml:/app/config.toml \
  ghcr.io/t3-labs/edge-video:latest
```

### 6. Verificar Status

```bash
# Ver logs
docker logs -f edge-video

# Verificar conectividade
docker exec edge-video ping -c 2 rabbitmq
docker exec edge-video ping -c 2 redis
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

# Verificar frames no Redis (formato v1.2.0+)
redis-cli -h localhost -p 6379 KEYS "guard_vhost:frames:*"

# Ver detalhes de uma chave
redis-cli -h localhost -p 6379 GET "guard_vhost:frames:cam1:1731024000123456789:00001"

# Contar frames armazenados
redis-cli -h localhost -p 6379 DBSIZE
```

!!! info "Formato de Chaves Redis"
    A partir da v1.2.0, o formato é: `{vhost}:{prefix}:{cameraID}:{unix_nano}:{sequence}`
    
    Exemplo: `supermercado_vhost:frames:cam4:1731024000123456789:00001`
    
    [Saiba mais sobre o formato otimizado](../features/redis-storage.md)

## Próximos Passos

<div class="grid cards" markdown>

-   :material-cog:{ .lg } __Configuração__
    
    Configure suas câmeras e ajuste parâmetros
    
    [:octicons-arrow-right-24: Configurar](configuration.md)

-   :material-rocket-launch:{ .lg } __Quick Start__
    
    Comece a usar em menos de 5 minutos
    
    [:octicons-arrow-right-24: Quick Start](quickstart.md)

-   :material-memory:{ .lg } __Redis Storage__
    
    Entenda o cache de frames otimizado
    
    [:octicons-arrow-right-24: Redis Guide](../features/redis-storage.md)

-   :material-bug:{ .lg } __Troubleshooting__
    
    Resolva problemas comuns
    
    [:octicons-arrow-right-24: Troubleshooting](../guides/troubleshooting.md)

</div>

## Desinstalação

### Docker Compose

```bash
# Parar serviços
docker-compose stop

# Parar e remover containers
docker-compose down

# Remover volumes (apaga dados Redis/RabbitMQ)
docker-compose down -v

# Remover tudo incluindo imagens
docker-compose down -v --rmi all
```

### Docker Manual

```bash
# Parar containers
docker stop edge-video rabbitmq redis redisinsight

# Remover containers
docker rm edge-video rabbitmq redis redisinsight

# Remover rede
docker network rm edge-video-network

# Remover imagens (opcional)
docker rmi ghcr.io/t3-labs/edge-video:latest
docker rmi rabbitmq:3.13-management-alpine
docker rmi redis:7-alpine
```

### Local

```bash
# Parar serviços
sudo systemctl stop rabbitmq-server
sudo systemctl stop redis-server

# Desabilitar autostart
sudo systemctl disable rabbitmq-server
sudo systemctl disable redis-server

# Remover binário
rm -f edge-video

# Remover dependências (cuidado!)
sudo apt remove --purge rabbitmq-server redis-server ffmpeg
```

## Atualizando

### Docker Compose

```bash
# Pull das imagens mais recentes
docker-compose pull

# Recriar containers
docker-compose up -d --force-recreate
```

### Docker Manual

```bash
# Pull nova versão
docker pull ghcr.io/t3-labs/edge-video:latest

# Parar container antigo
docker stop edge-video
docker rm edge-video

# Iniciar novo container
docker run -d \
  --name edge-video \
  --network edge-video-network \
  -v $(pwd)/config.toml:/app/config.toml \
  ghcr.io/t3-labs/edge-video:latest
```

### Build Local

```bash
# Atualizar código
git pull origin main

# Rebuild
go build -o edge-video ./cmd/edge-video

# Reiniciar serviço
./edge-video
```

!!! warning "Breaking Changes v1.2.0"
    Ao atualizar para v1.2.0+, o formato de chaves Redis mudou de RFC3339 para Unix nanoseconds.
    
    **Opções de migração:**
    
    1. **FLUSHDB** - Limpar Redis (recomendado para dev/staging)
    2. **Aguardar TTL** - Deixar chaves antigas expirarem (recomendado para produção)
    3. **Script de migração** - Converter chaves existentes
    
    [Veja o guia completo de migração](../features/redis-storage.md#migracao)

## Ambientes de Deployment

### Desenvolvimento Local

```bash
# docker-compose.yml mínimo
version: '3.8'
services:
  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    ports:
      - "5672:5672"
      - "15672:15672"
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  edge-video:
    image: ghcr.io/t3-labs/edge-video:latest
    volumes:
      - ./config.toml:/app/config.toml
    depends_on:
      - rabbitmq
      - redis
```

### Staging/Produção

```bash
# docker-compose.prod.yml
version: '3.8'
services:
  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    environment:
      - RABBITMQ_DEFAULT_USER=${RABBITMQ_USER}
      - RABBITMQ_DEFAULT_PASS=${RABBITMQ_PASS}
    volumes:
      - rabbitmq-data:/var/lib/rabbitmq
    restart: unless-stopped
    healthcheck:
      test: rabbitmq-diagnostics ping
      interval: 30s
      timeout: 10s
      retries: 3
  
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASS}
    volumes:
      - redis-data:/data
    restart: unless-stopped
    healthcheck:
      test: redis-cli --pass ${REDIS_PASS} ping
      interval: 30s
      timeout: 10s
      retries: 3
  
  edge-video:
    image: ghcr.io/t3-labs/edge-video:${VERSION:-latest}
    volumes:
      - ./config.toml:/app/config.toml:ro
    environment:
      - REDIS_PASSWORD=${REDIS_PASS}
    depends_on:
      rabbitmq:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G

volumes:
  rabbitmq-data:
  redis-data:
```

### Kubernetes (Básico)

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-video
spec:
  replicas: 3
  selector:
    matchLabels:
      app: edge-video
  template:
    metadata:
      labels:
        app: edge-video
    spec:
      containers:
      - name: edge-video
        image: ghcr.io/t3-labs/edge-video:latest
        resources:
          limits:
            cpu: "2"
            memory: "2Gi"
          requests:
            cpu: "1"
            memory: "1Gi"
        volumeMounts:
        - name: config
          mountPath: /app/config.toml
          subPath: config.toml
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
      volumes:
      - name: config
        configMap:
          name: edge-video-config
---
apiVersion: v1
kind: Service
metadata:
  name: edge-video
spec:
  selector:
    app: edge-video
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 8080
```

## Requisitos por Escala

### Pequena Escala (1-5 câmeras)

| Recurso | Mínimo | Recomendado |
|---------|--------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 2 GB | 4 GB |
| Disco | 10 GB | 20 GB |
| Rede | 10 Mbps | 50 Mbps |

### Média Escala (6-20 câmeras)

| Recurso | Mínimo | Recomendado |
|---------|--------|-------------|
| CPU | 4 cores | 8 cores |
| RAM | 4 GB | 8 GB |
| Disco | 20 GB | 50 GB |
| Rede | 50 Mbps | 100 Mbps |

### Grande Escala (20+ câmeras)

| Recurso | Mínimo | Recomendado |
|---------|--------|-------------|
| CPU | 8 cores | 16 cores |
| RAM | 8 GB | 16 GB |
| Disco | 50 GB | 100 GB SSD |
| Rede | 100 Mbps | 1 Gbps |

!!! tip "Escalabilidade Horizontal"
    Para mais de 50 câmeras, considere:
    
    - **Múltiplas instâncias** com load balancing
    - **Redis Cluster** para cache distribuído
    - **RabbitMQ Cluster** para alta disponibilidade
    - **Kubernetes** para orquestração automática
