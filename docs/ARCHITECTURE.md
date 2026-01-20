# Edge Video V2 - Arquitetura

## Pipeline

```
STARTUP → Camera Registration API (POST)
   ↓
CÂMERA → FFmpeg → Redis.SET(JPEG) + RabbitMQ.Publish(redis_key) → Consumer busca Redis → Processa
```

**Latência**: ~160ms (90% Redis remoto)
**Gargalo**: Redis usado como blob storage (deveria ser metadata)

## Camera Registration API

Antes de criar exchanges RabbitMQ, o producer faz POST para API externa registrando câmeras.

**Configuração** (`config.yaml`):
```yaml
camera_registration:
  enabled: true
  url: "https://your-api.com/camera"
```

**Payload**:
```json
{
  "cameras": [{"id": "cam1", "url": "rtmp://..."}],
  "namespace": "vhost",
  "rabbitmq_url": "amqp://...",
  "routing_key": "prefix",
  "exchange": "exchange_name",
  "vhost": "vhost"
}
```

**Sequência de startup**:
1. Carrega config
2. Cleanup FFmpeg órfãos
3. Inicia metrics server
4. Conecta Redis
5. **Registra câmeras na API** ← NOVO
6. Inicia pprof
7. Cria publishers + exchanges
8. Inicia câmeras

**Resiliência**: Se API falhar, loga aviso mas continua normalmente (não quebra startup).

## Componentes

```
cmd/producer/main.go           - Entry point
internal/
  ├─ stream/camera_stream.go   - FFmpeg + Latest Frame Policy
  ├─ messaging/publisher.go    - RabbitMQ auto-reconnect + ACK/NACK
  ├─ storage/redis_client.go   - Redis com retry
  ├─ resilience/circuit_breaker.go - Proteção falhas
  ├─ memory/controller.go      - RAM limits
  └─ monitoring/metrics.go     - Prometheus
```

## Latest Frame Policy

`frameChan` buffer=5. Se cheio, descarta frame antigo (não bloqueia).
Garante sincronização perfeita (sempre latest).

## Buffer Pool LOCAL

Cada câmera tem `sync.Pool` dedicado (512KB buffers).
**Crítico**: NUNCA usar pool global (causa frame contamination).

## Circuit Breaker

- Max failures: 5 → OPEN
- Reset timeout: 30s → HALF_OPEN
- Backoff: 5s → 5min (exponencial)

## Memory

**Consumo (6 câmeras)**:
- FFmpeg: 450 MB
- Buffers: 50 MB
- Runtime: 58 MB
- **Total: ~558 MB**

## Métricas Prometheus

**Por câmera**:
- `edge_video_frames_received_total`
- `edge_video_frames_published_total`
- `edge_video_frames_dropped_total`
- `edge_video_camera_fps`
- `edge_video_circuit_breaker_state` (0=CLOSED, 1=OPEN)

**Globais**:
- `edge_video_publisher_confirms_ack_total`
- `edge_video_system_cpu_percent`
- `edge_video_system_memory_mb`
- `edge_video_goroutines_count`

## Performance Real

- FPS: 7-9 (target: 15) → Redis latency
- Throughput: 11.6 MB/s
- Frame drops: 0%
- Taxa sucesso: 100%

## Redis Key Format

```
vhost:prefix:camera:timestamp
supercarlao_rj_mercado:frames:cam2:1765346747233190500
```

TTL=120s, armazena JPEG completo (50-330 KB).

## Publisher Confirms

Rastreia 100% ACK/NACK via `handleConfirms()` goroutine.
Cada reconexão RabbitMQ **DEVE** parar goroutine antigo antes de criar novo (evita leak).

## Frame Sizes

- cam1 (RTMP): 320 KB
- cam2 (RTSP): 64 KB
- cam3 (RTSP): 180 KB
- cam4 (RTSP): 115 KB
- cam5 (RTSP): 97 KB

**Média**: 155 KB/frame
**Throughput total**: 11.6 MB/s @ 15 FPS
