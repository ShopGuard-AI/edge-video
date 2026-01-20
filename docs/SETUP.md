# Edge Video V2 - Setup & Config

## Quick Start

```bash
# Build
go build -o producer.exe cmd/producer/main.go

# Run
./producer.exe
```

**Métricas**: http://localhost:2112/metrics
**pprof**: http://localhost:6060/debug/pprof/

## config.yaml Essencial

```yaml
fps: 15
quality: 5

redis:
  enabled: true
  address: "35.199.96.88:6379"
  password: "senha"
  ttl: 120

circuit_breaker:
  enabled: true
  max_failures: 5
  reset_timeout: 30s

memory_controller:
  enabled: true
  max_memory_mb: 1024  # 6 câmeras = ~558 MB

cameras:
  - id: "cam1"
    url: "rtmp://host:1935/stream"
    exchange: "vhost.exchange"
    routing_key: "vhost.cam1"
```

## Consumer Python

```bash
cd consumer
pip install -r requirements.txt
python src/consumer.py
```

Consumer config (`consumer/config.yaml`):
```yaml
rabbitmq:
  queue: "vhost.frames.cam1"
  arguments:
    x-message-ttl: 2000  # Previne backlog infinito
    x-max-length: 100
```

## Frame Sizes

| Câmera | Tamanho | Throughput @ 15 FPS |
|--------|---------|---------------------|
| cam1   | 320 KB  | 4.8 MB/s            |
| cam2   | 64 KB   | 0.96 MB/s           |
| cam3   | 180 KB  | 2.7 MB/s            |
| cam4   | 115 KB  | 1.73 MB/s           |
| cam5   | 97 KB   | 1.46 MB/s           |

**Total**: 11.6 MB/s

## Monitoring

```bash
cd monitoring
docker-compose up -d
```

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

## Testes

```bash
# Single camera
./producer.exe  # config com 1 câmera

# All cameras
scripts/test_all_cameras.bat

# E2E
cd tests/e2e && go test -v

# Integration
cd tests/integration && go test -v
```

## Troubleshooting

**FPS baixo**: Redis remoto → Use local ou aumente TTL
**Memory > 1GB**: Reduza quality (5→3) ou câmeras
**Circuit Breaker OPEN**: URL inválida → Teste com VLC
**Goroutine leak**: http://localhost:6060/debug/pprof/goroutine?debug=1
**JPEG decode fail**: Fallback PIL no consumer
