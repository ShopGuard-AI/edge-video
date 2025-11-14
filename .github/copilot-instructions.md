# Edge Video - AI Coding Agent Instructions

## Project Overview

**Edge Video** is a distributed RTSP camera capture system designed for edge computing environments. It captures frames from multiple IP cameras, stores them in Redis with optimized keys, and distributes metadata events via RabbitMQ for real-time processing by multiple consumers.

**Tech Stack**: Go 1.24, FFmpeg (RTSP capture), Redis 7.x, RabbitMQ 3.13, Python 3.11+ (consumers)

**Platforms**: Linux (Docker), Windows (Native Service), macOS (Development)

## Critical Architecture Patterns

### 1. Multi-Tenant Vhost-Based Isolation

The system uses **RabbitMQ vhosts as tenant identifiers** throughout the stack:

- **Config**: AMQP URL contains vhost: `amqp://user:pass@host:5672/supermercado_vhost`
- **Extraction**: `config.ExtractVhostFromAMQP()` parses vhost from URL (returns "/" if missing)
- **Redis Keys**: Format `{vhost}:{prefix}:{camera}:{unix_nano}:{seq}`
- **Example**: `supermercado_vhost:frames:cam1:1731073800123456789:00001`

**Why**: Guarantees data isolation between tenants without separate Redis instances or manual `instance_id` configuration.

**Files**: `pkg/config/config.go`, `internal/storage/key_generator.go`, `cmd/edge-video/main.go:36-38`

### 2. Unix Nanoseconds Timestamp Format (v1.2.0+)

**BREAKING CHANGE**: Migrated from RFC3339 strings to Unix nanoseconds (int64) for 36% smaller keys and 10x faster comparisons.

```go
// CORRECT (v1.2.0+)
timestamp := time.Now().UnixNano()  // 1731073800123456789 (int64)
key := fmt.Sprintf("%s:frames:%s:%d:%s", vhost, cameraID, timestamp, sequence)

// DEPRECATED (v1.1.x)
timestamp := time.Now().Format(time.RFC3339Nano)
```

**Impact**: All Redis operations, metadata JSON, and consumers must use `timestamp_nano` (int64) field.

**Files**: `internal/storage/key_generator.go`, `internal/metadata/publisher.go`, `docs/features/redis-storage.md`

### 3. Metadata-First Architecture

**Frame Flow**:
1. Camera → FFmpeg captures JPEG frame
2. Frame stored in Redis (key: `{vhost}:frames:{cam}:{unix_nano}:{seq}`, TTL: 300s)
3. **Metadata event** published to RabbitMQ (~200 bytes JSON)
4. Consumers receive lightweight notification, fetch frame only if needed

**Metadata JSON Structure**:
```json
{
  "camera_id": "cam1",
  "timestamp": "2024-11-08T14:30:00.123456789Z",
  "timestamp_nano": 1731073800123456789,
  "sequence": "00001",
  "redis_key": "meu-cliente:frames:cam1:1731073800123456789:00001",
  "vhost": "meu-cliente",
  "frame_size_bytes": 245678,
  "ttl_seconds": 300
}
```

**Why**: Decouples frame storage from delivery. Consumers decide if they need frame data (AI: yes, monitoring: no).

**Files**: `internal/metadata/publisher.go`, `pkg/camera/camera.go:280-310`, `test_camera_redis_amqp.py`

### 4. Worker Pool + Circuit Breaker Pattern

**Concurrency Model**:
- **Worker Pool**: Processes frame capture/publish jobs asynchronously (`pkg/worker/`)
- **Frame Buffer**: Bounded queue prevents memory overflow (`pkg/buffer/`)
- **Circuit Breaker**: Stops retrying failed cameras after N failures (`pkg/circuit/`)

**Config**:
```toml
[optimization]
max_workers = 20           # Concurrent frame processors
buffer_size = 200          # Max queued frames
circuit_max_failures = 5   # Open circuit after 5 failures
circuit_reset_seconds = 60 # Try reconnect after 60s
```

**Files**: `cmd/edge-video/main.go:51-55`, `pkg/camera/camera.go:36-60`

## Project Structure

```
cmd/edge-video/main.go        # Entry point, wires dependencies
cmd/edge-video-service/       # Windows service wrapper
  ├── main.go                 # Service entry point with Windows SVC interface
  ├── service.go              # Service management (install/uninstall/start/stop)
  └── service_stub.go         # Non-Windows platform stubs
pkg/                          # Public packages (camera, config, mq, etc.)
  ├── camera/                 # FFmpeg RTSP capture logic
  ├── config/                 # TOML config parsing + vhost extraction
  ├── mq/                     # RabbitMQ (amqp.go) and MQTT (mqtt.go) publishers
  ├── registration/           # API registration client with retry logic
  ├── worker/                 # Worker pool for async frame processing
  ├── circuit/                # Circuit breaker for failed cameras
  └── buffer/                 # Bounded frame buffer
internal/                     # Private packages (not exported)
  ├── storage/                # Redis store + KeyGenerator (vhost-aware)
  └── metadata/               # Metadata publisher (JSON events to RabbitMQ)
.github/workflows/
  └── windows-installer.yml   # CI/CD for Windows installer
installer/windows/
  ├── edge-video-installer.nsi # NSIS installer script
  └── edge-video.ico          # Application icon
docs/                         # MkDocs documentation
  ├── windows/                # Windows-specific documentation
  └── ci-cd-windows.md        # Windows CI/CD guide
test_camera_redis_amqp.py     # Reference Python consumer implementation
build-windows.sh              # Cross-compilation script for Windows
```

## Windows Service Architecture

Edge Video supports native Windows deployment as a Windows Service, providing:

### Service Features
- **Auto-start**: Starts automatically with Windows
- **Background execution**: Runs without user session
- **Event logging**: Logs to Windows Event Log (Application → EdgeVideoService)
- **Service recovery**: Auto-restart on failures
- **Admin management**: Install/uninstall via elevated privileges

### Service Structure
```
C:\Program Files\T3Labs\EdgeVideo\
├── edge-video-service.exe    # Windows service binary
├── edge-video.exe           # CLI binary for debugging
├── config\
│   └── config.toml          # Service configuration
├── logs\                    # Local logs (optional)
└── uninstall.exe           # Generated by installer
```

### Service Management Commands
```cmd
# Install service
edge-video-service.exe install

# Service control
edge-video-service.exe start|stop
net start|stop EdgeVideoService
Services.msc → Edge Video Camera Capture Service

# Debug mode
edge-video-service.exe console

# Uninstall
edge-video-service.exe uninstall
```

### Windows CI/CD Pipeline

**GitHub Actions Workflow** (`.github/workflows/windows-installer.yml`):
1. **Build Environment**: `windows-latest` with Go 1.24 + NSIS
2. **Cross-compilation**: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`
3. **Installer Generation**: NSIS creates `EdgeVideoSetup-X.X.X.exe`
4. **Artifacts**: Service binary, CLI binary, installer, SHA256 checksums
5. **Auto-release**: On Git tags, creates GitHub Release with artifacts

**Triggers**: Git tags (`v*`), main branch, PRs, manual dispatch

**Build Matrix**: Currently Windows x64 only, extensible to x86/ARM64

## Development Workflows

### Build & Run Locally

```bash
# Build binary
go build -o edge-video ./cmd/edge-video

# Run with custom config
./edge-video --config config.toml

# Run tests with race detector
go test -v -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Docker Compose Development

```bash
# Build and start all services (Redis, RabbitMQ, camera-collector)
docker-compose up --build

# Use custom config
CONFIG_PATH=./my-config.toml docker-compose up

# Check logs
docker logs -f camera-collector

# Access RabbitMQ Management UI: http://localhost:15672 (user/password)
# Access RedisInsight: http://localhost:5540
```

### Testing Consumers

```bash
# Python consumer with Redis + RabbitMQ
python3 test_camera_redis_amqp.py

# Visualize frames with OpenCV (requires X11/display)
# Set enable_visualization=True in script
```

## Configuration Conventions

### Multi-Camera Setup

Each camera is a separate Go instance with its own `config.toml`:

```toml
# cam1.toml
[camera]
[[cameras]]
id = "cam1"
url = "rtsp://admin:pass@192.168.1.100:554/stream1"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/meu-cliente"  # Same vhost for tenant

# Run: ./edge-video --config cam1.toml
```

**Why separate instances**: Isolates failures, simplifies scaling, enables per-camera optimization.

### Protocol Selection

```toml
protocol = "amqp"  # or "mqtt"

[amqp]
amqp_url = "amqp://user:pass@host:5672/vhost"
exchange = "cameras"
routing_key_prefix = "camera."  # Final: camera.cam1

[mqtt]
broker = "tcp://localhost:1883"
topic_prefix = "camera/"  # Final: camera/cam1
```

### Redis Storage (Optional)

```toml
[redis]
enabled = true          # If false, only publishes to MQ (no storage)
address = "redis:6379"
ttl_seconds = 300       # Frame expiration (5 min)
prefix = "frames"       # Key component: {vhost}:frames:{cam}...
```

### API Registration (Optional)

The system can register itself with an external API on startup, sending configuration data:

```toml
[registration]
enabled = true
api_url = "http://api.example.com/register"
```

**Registration payload** (JSON POST):
```json
{
  "cameras": [
    {"id": "cam1", "url": "rtsp://..."},
    {"id": "cam2", "url": "rtsp://..."}
  ],
  "namespace": "supermercado_vhost",
  "rabbitmq_url": "amqp://user:pass@rabbitmq:5672/supermercado_vhost",
  "routing_key": "camera.",
  "exchange": "cameras",
  "vhost": "supermercado_vhost"
}
```

**Retry behavior**: If registration fails, retries every 1 minute until successful (runs in background goroutine).

**Files**: `pkg/registration/client.go`, `cmd/edge-video/main.go:60-62`

## Testing Patterns

### Unit Tests

```go
// Test vhost extraction
func TestExtractVhostFromAMQP(t *testing.T) {
    cfg := &Config{AMQP: AMQPConfig{AmqpURL: "amqp://user:pass@host:5672/my_vhost"}}
    assert.Equal(t, "my_vhost", cfg.ExtractVhostFromAMQP())
}
```

**Run**: `go test ./pkg/config/... -v`

### Key Generator Tests

Validates vhost isolation, concurrency safety, sequence uniqueness:

```bash
go test ./internal/storage/... -v -race
```

**Files**: `internal/storage/key_generator_test.go` (15 tests + 3 benchmarks)

## Integration Points

### Consumer Pattern (Python)

```python
import redis
import pika
import json

# 1. Subscribe to metadata exchange
channel.exchange_declare(exchange='camera.metadata', exchange_type='topic', durable=True)
channel.queue_bind(exchange='camera.metadata', queue=queue, routing_key='camera.metadata.event')

# 2. On metadata event, extract redis_key
def callback(ch, method, properties, body):
    metadata = json.loads(body)
    redis_key = metadata['redis_key']  # e.g., "vhost:frames:cam1:1731073800123456789:00001"
    
    # 3. Fetch frame from Redis
    frame_bytes = redis_client.get(redis_key)
```

**Reference Implementation**: `test_camera_redis_amqp.py` (CameraFrameConsumer class)

### Go Publisher Pattern

```go
// Publish frame to RabbitMQ/MQTT
err := publisher.Publish(ctx, cameraID, frameData)

// Save to Redis and publish metadata
key, err := redisStore.SaveFrame(ctx, cameraID, timestamp, frameData)
metaPublisher.PublishMetadata(cameraID, timestamp, key, width, height, size, "jpeg")
```

**Files**: `pkg/camera/camera.go:259-310`, `pkg/mq/amqp.go`, `pkg/mq/mqtt.go`

## Common Gotchas

1. **Vhost must match**: AMQP URL vhost must be consistent across all services for tenant isolation
2. **Unix nano everywhere**: Always use `timestamp_nano` (int64), not RFC3339 strings (deprecated in v1.2.0)
3. **Sequence prevents collisions**: KeyGenerator adds sequence suffix (00001, 00002...) for frames captured in same nanosecond
4. **TTL tuning**: 300s (5 min) is default. Adjust based on processing latency and memory constraints
5. **FFmpeg required**: Docker image includes FFmpeg. Local development needs: `apt install ffmpeg` or `brew install ffmpeg`
6. **One instance per camera**: Don't configure multiple cameras in same `config.toml` for production (use separate processes)

## Documentation

MkDocs site source: `docs/`

**Key pages**:
- `docs/features/redis-storage.md` - Redis key format, TTL, queries
- `docs/features/camera-capture.md` - RTSP protocols, FPS, retry logic
- `docs/features/message-queue.md` - RabbitMQ setup, vhost configuration
- `docs/features/metadata.md` - Metadata publisher architecture

**Build docs locally**:
```bash
pip install -r requirements-docs.txt
mkdocs serve  # http://localhost:8000
```

## Metrics & Observability

Prometheus metrics exposed on `:2112/metrics`:

```go
metrics.FramesProcessed.WithLabelValues(cameraID).Inc()
metrics.PublishLatency.WithLabelValues("amqp").Observe(duration.Seconds())
```

**Files**: `pkg/metrics/metrics.go`, `cmd/edge-video/main.go:177-183`

## When Modifying Code

- **Config changes**: Update `pkg/config/config.go` struct + add tests in `pkg/config/config_test.go`
- **New metadata fields**: Update `internal/metadata/publisher.go` Metadata struct + all consumers
- **Redis key format**: Modify `internal/storage/key_generator.go` + update docs + add migration guide
- **New camera protocols**: Extend `pkg/camera/camera.go` FFmpeg command builder
- **Breaking changes**: Document in `CHANGELOG.md` under "Breaking Changes" section

## Quick Reference Commands

```bash
# Run with race detector
go run -race ./cmd/edge-video --config config.toml

# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run

# Generate mocks
# (Pattern: create mocks in pkg/mq/publisher_mock.go for interfaces)

# Check Redis keys
redis-cli KEYS "supermercado_vhost:frames:*"

# Monitor RabbitMQ queue
# Management UI: http://localhost:15672 → Queues tab
```
