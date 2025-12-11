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

### Métricas Prometheus

**Por câmera**:
- `edge_video_frames_received_total`
- `edge_video_frames_published_total`
- `edge_video_frames_dropped_total`
- `edge_video_camera_fps`
- `edge_video_circuit_breaker_state` (0=CLOSED, 1=OPEN, 2=HALF_OPEN)

**Globais**:
- `edge_video_publisher_confirms_ack_total`
- `edge_video_system_cpu_percent`
- `edge_video_system_memory_mb`
- `edge_video_goroutines_count`

### Stack Grafana

```bash
cd monitoring && docker-compose up -d
```

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

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

## 📚 Docs

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Pipeline, componentes, métricas
- **[docs/BUGS_FIXED.md](docs/BUGS_FIXED.md)** - Bugs críticos + soluções
- **[docs/SETUP.md](docs/SETUP.md)** - Setup, config, troubleshooting

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

```bash
# Build
go build -o producer.exe cmd/producer/main.go
make build-prod   # Otimizado

# Deploy
# Copie producer.exe + config.yaml para servidor
# Configure como serviço (systemd/Windows Service)
```

---

## 📞 Support

- **Issues**: GitHub issue com logs (sem senhas)
- **pprof**: http://localhost:6060/debug/pprof/
- **Metrics**: http://localhost:2112/metrics

---

**Versão**: V2.3 | **Status**: ✅ Production-Ready | **Update**: 2024-12-11
