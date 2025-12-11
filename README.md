# Edge Video V2 - Production-Ready

Sistema de captura e streaming de múltiplas câmeras RTSP/RTMP com armazenamento Redis e mensageria RabbitMQ.

**Producer (Go)** captura frames via FFmpeg → armazena JPEGs no Redis → publica redis_key no RabbitMQ.
**Consumer (Python)** consome redis_key → busca JPEG do Redis → processa com OpenCV/YOLO.

---

## 🚀 Quick Start

### 1. Build

```bash
go build -o producer.exe cmd/producer/main.go
```

### 2. Config `config.yaml`

```yaml
fps: 15
quality: 5

redis:
  enabled: true
  address: "35.199.96.88:6379"
  password: "senha"
  ttl: 120

amqp:
  url: "amqp://user:pass@host:5672/vhost"

cameras:
  - id: "cam1"
    url: "rtmp://servidor:1935/rtmp/stream"
    exchange: "vhost.exchange"
    routing_key: "vhost.cam1"
```

### 3. Run

```bash
./producer.exe          # http://localhost:2112/metrics
cd consumer && python src/consumer.py
```

---

## 📐 Arquitetura

```
CÂMERA → FFmpeg → PRODUCER → Redis.SET(JPEG) + RabbitMQ.Publish(key) → CONSUMER → Redis.GET → Processa
```

**Latência**: ~160ms (90% Redis remoto)
**Redis Key**: `vhost:frames:camera:timestamp` (JPEG 50-330 KB, TTL=120s)

---

## 🏗️ Estrutura

```
edge-video/
├── cmd/producer/main.go       - Entry point
├── internal/                  - Core Go (config, stream, messaging, storage, resilience, memory, monitoring)
├── consumer/                  - Python consumer
├── tests/                     - E2E + Integration
├── docs/                      - ARCHITECTURE.md, BUGS_FIXED.md, SETUP.md
├── monitoring/                - Grafana + Prometheus
├── config.yaml                - Config principal
└── producer.exe               - Binary compilado
```

---

## ⚡ Features

### Latest Frame Policy
Buffer=5. Se cheio, descarta antigo (não bloqueia). Garante sync perfeita.

### Buffer Pool LOCAL
Cada câmera tem `sync.Pool` dedicado (512KB). **CRÍTICO**: Nunca global (causa contamination).

### Circuit Breaker
- Max failures: 5 → OPEN
- Reset: 30s → HALF_OPEN
- Backoff: 5s → 5min (exponencial)

### Memory Controller
- Warning: 50% (512 MB)
- Critical: 70% (716 MB)
- Emergency: 85% (870 MB)
- **Consumo 6 câmeras**: ~558 MB

### Publisher Confirms
Rastreia 100% ACK/NACK RabbitMQ. Reconexão DEVE parar goroutine antigo (previne leak).

---

## 📊 Performance

**Config**: 5 câmeras, FPS=15, quality=5

| Métrica | Valor |
|---------|-------|
| FPS real | 7-9 (Redis latency) |
| Throughput | 11.6 MB/s |
| Latência | 163ms/frame |
| Drops | 0% |
| Sucesso | 100% |

### Frame Sizes

| Câmera | Protocolo | Size | Throughput@15FPS |
|--------|-----------|------|------------------|
| cam1   | RTMP      | 320KB| 4.8 MB/s         |
| cam2   | RTSP      | 64KB | 0.96 MB/s        |
| cam3   | RTSP      | 180KB| 2.7 MB/s         |
| cam4   | RTSP      | 115KB| 1.73 MB/s        |
| cam5   | RTSP      | 97KB | 1.46 MB/s        |

---

## 📈 Monitoramento

Producer expõe métricas Prometheus em **http://localhost:2112/metrics**.

### Métricas Disponíveis

**Por câmera**:
- `edge_video_frames_received_total` - Total de frames recebidos do FFmpeg
- `edge_video_frames_published_total` - Total de frames publicados no RabbitMQ
- `edge_video_frames_dropped_total` - Total de frames descartados (buffer cheio)
- `edge_video_camera_fps` - FPS real da câmera
- `edge_video_publish_latency_ms` - Latência de publicação em ms
- `edge_video_circuit_breaker_state` - Estado do circuit breaker (0=CLOSED, 1=OPEN, 2=HALF_OPEN)

**Globais do sistema**:
- `edge_video_publisher_confirms_ack_total` - Total de ACKs do RabbitMQ
- `edge_video_publisher_confirms_nack_total` - Total de NACKs do RabbitMQ
- `edge_video_system_cpu_percent` - Uso de CPU do processo (%)
- `edge_video_system_ram_mb` - Uso de RAM em MB
- `edge_video_system_goroutines` - Número de goroutines ativas
- `edge_video_uptime_seconds` - Tempo de execução em segundos

### Stack Prometheus + Grafana (Opcional)

```bash
cd monitoring
docker-compose up -d
```

**Serviços**:
- 📊 **Grafana**: http://localhost:3000 (admin/admin)
- 📈 **Prometheus**: http://localhost:9090

**Dashboard Grafana pré-configurado** com:
- 📈 FPS em tempo real por câmera
- 💾 Uso de memória e CPU
- 🔴 Circuit Breaker states
- 📊 Frames publicados vs descartados
- ⏱️ Latência de publicação
- 🎯 Taxa de ACK/NACK RabbitMQ
- 📉 Gráficos históricos (30 dias)

Para mais detalhes: `monitoring/README.md`

---

## ⚙️ Config Essencial

### Circuit Breaker

```yaml
circuit_breaker:
  enabled: true
  max_failures: 5
  reset_timeout: 30s
  initial_backoff: 5s
  max_backoff: 5m
```

### Memory

```yaml
memory_controller:
  enabled: true
  max_memory_mb: 1024
  warning_percent: 50.0
  critical_percent: 70.0
  gc_trigger_percent: 60.0
```

### Consumer TTL

```yaml
rabbitmq:
  arguments:
    x-message-ttl: 2000     # Warm-up: 120s→2s (98% melhoria)
    x-max-length: 100
```

---

## 🧪 Testes

```bash
./producer.exe                    # Single camera (config com 1 câmera)
scripts/test_all_cameras.bat     # All cameras
cd tests/e2e && go test -v       # E2E
cd tests/integration && go test -v # Integration
```

---

## 🔧 Troubleshooting

| Problema | Solução |
|----------|---------|
| FPS baixo | Redis local ou aumente TTL |
| Memory > 1GB | Reduza quality (5→3) |
| Circuit Breaker OPEN | Teste URL com VLC/FFmpeg |
| Goroutine leak | http://localhost:6060/debug/pprof/goroutine?debug=1 |
| JPEG decode fail | Fallback PIL no consumer |

---

## 🐛 Bugs Críticos Corrigidos

### Bug #1: Frame Cross-Contamination (V2.1)
- **Sintoma**: Frames de cam2 em cam1 (5-15%)
- **Causa**: `sync.Pool` GLOBAL compartilhado
- **Fix**: Pool LOCAL por câmera

### Bug #2: Goroutine Leak (V2.2)
- **Sintoma**: Goroutines acumulam pós-reconexão
- **Causa**: `handleConfirms()` não parado
- **Fix**: Canal `confirmsDone`

Detalhes: `docs/BUGS_FIXED.md`

---

## 📚 Documentação

### Deployment
- **[DEPLOY_WINDOWS.md](DEPLOY_WINDOWS.md)** - Deploy Windows (NSSM service + Docker)
- **[DEPLOY_UBUNTU.md](DEPLOY_UBUNTU.md)** - Deploy Ubuntu (systemd service + Docker)

### Técnica
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Pipeline, componentes, métricas
- **[docs/BUGS_FIXED.md](docs/BUGS_FIXED.md)** - Bugs críticos + soluções
- **[docs/SETUP.md](docs/SETUP.md)** - Setup, config, troubleshooting

### Monitoramento
- **[monitoring/README.md](monitoring/README.md)** - Grafana + Prometheus + dashboards

---

## 🎯 Performance Tuning

- **Latency baixa**: Redis local
- **Throughput alto**: quality=3, FPS↑
- **RAM baixa**: Reduza buffer_size

---

## 🔑 Requisitos

- Go 1.23+
- Python 3.11+ (consumer)
- FFmpeg no PATH
- Redis 6.0+
- RabbitMQ 3.8+
- Docker (opcional, monitoring)

---

## 📦 Build & Deploy

### Build Local

```bash
# Windows
go build -ldflags="-s -w" -o producer.exe cmd/producer/main.go

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o producer-linux cmd/producer/main.go

# Makefile
make windows    # Build producer.exe
make linux      # Build producer-linux
make release    # Organiza build/windows/ e build/linux/
```

### Deploy Production

**Windows**: Ver **[DEPLOY_WINDOWS.md](DEPLOY_WINDOWS.md)**
- Setup automático FFmpeg (setup.ps1)
- NSSM service resiliente
- Auto-restart em falhas
- Docker Desktop + Grafana (opcional)

**Ubuntu/Linux**: Ver **[DEPLOY_UBUNTU.md](DEPLOY_UBUNTU.md)**
- Setup automático FFmpeg (setup-ubuntu.sh)
- Systemd service com security hardening
- Logs centralizados journald
- Docker + Docker Compose + Grafana (opcional)

### Releases

Download: https://github.com/ShopGuard-AI/edge-video/releases

- `edge-video-v1.6-windows.zip` - Producer + setup + docs
- `edge-video-v1.6-linux.tar.gz` - Producer + setup + docs

---

## 📞 Support

- **Issues**: GitHub issue com logs (sem senhas)
- **pprof**: http://localhost:6060/debug/pprof/
- **Metrics**: http://localhost:2112/metrics

---

**Versão**: v1.6 | **Status**: ✅ Production-Ready | **Update**: 2025-12-11
