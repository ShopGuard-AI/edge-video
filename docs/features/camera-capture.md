# Captura de Câmeras

O **Edge Video** captura frames de câmeras IP (RTSP/HTTP) em tempo real, processando e distribuindo os dados através de RabbitMQ e Redis para aplicações consumidoras.

## Visão Geral

O Camera Capture oferece:

- ✅ **Protocolo RTSP**: Suporte nativo a câmeras IP
- ✅ **Múltiplas Câmeras**: Capture simultânea de várias fontes
- ✅ **Frame Rate Configurável**: Controle de FPS por câmera
- ✅ **Compressão Inteligente**: Otimização automática de tamanho
- ✅ **Reconexão Automática**: Retry logic para streams instáveis

## Arquitetura do Fluxo

```mermaid
sequenceDiagram
    participant Cam as Câmera IP
    participant CC as Camera Collector
    participant R as Redis
    participant MQ as RabbitMQ
    participant C as Consumer
    
    Cam->>CC: RTSP Stream
    CC->>CC: Captura Frame
    CC->>CC: Gera Timestamp Unix Nano
    CC->>R: Store (vhost:frames:cam:unix_nano:seq)
    CC->>MQ: Publish Metadata
    MQ->>C: Deliver Event
    C->>R: Fetch Frame (usando redis_key)
```

**Etapas do Processo:**

1. **Conexão RTSP**: Estabelece stream com câmera IP
2. **Captura**: Extrai frames em intervalos configurados
3. **Timestamp**: Gera Unix nanoseconds único (`time.UnixNano()`)
4. **Storage**: Armazena frame no Redis com TTL
5. **Notification**: Publica metadata no RabbitMQ
6. **Consumption**: Consumers processam conforme necessário

## Configuração

### Configuração Básica

=== "config.toml"

    ```toml
    [camera]
    # URL RTSP da câmera
    rtsp_url = "rtsp://admin:password@192.168.1.100:554/stream1"
    
    # Identificador único da câmera
    camera_id = "cam1"
    
    # FPS de captura (frames por segundo)
    fps = 5
    
    # Timeout de conexão (segundos)
    timeout = 10
    
    # Retry automático
    retry_enabled = true
    retry_max_attempts = 3
    retry_delay_seconds = 5
    
    [redis]
    enabled = true
    ttl_seconds = 300
    
    [metadata]
    enabled = true
    ```

=== "Docker Compose"

    ```yaml
    services:
      camera-collector-cam1:
        image: t3labs/edge-video:latest
        environment:
          - CAMERA_RTSP_URL=rtsp://admin:pass@192.168.1.100:554/stream1
          - CAMERA_ID=cam1
          - CAMERA_FPS=5
          - REDIS_ENABLED=true
          - METADATA_ENABLED=true
        networks:
          - edge-video
      
      camera-collector-cam2:
        image: t3labs/edge-video:latest
        environment:
          - CAMERA_RTSP_URL=rtsp://admin:pass@192.168.1.101:554/stream1
          - CAMERA_ID=cam2
          - CAMERA_FPS=10
        networks:
          - edge-video
    ```

=== "Multi-Camera"

    ```bash
    # Criar configuração para cada câmera
    
    # cam1.toml
    cat > cam1.toml << EOF
    [camera]
    rtsp_url = "rtsp://admin:pass@192.168.1.100:554/stream1"
    camera_id = "cam1"
    fps = 5
    
    [amqp]
    amqp_url = "amqp://user:pass@rabbitmq:5672/meu-cliente"
    EOF
    
    # cam2.toml
    cat > cam2.toml << EOF
    [camera]
    rtsp_url = "rtsp://admin:pass@192.168.1.101:554/stream1"
    camera_id = "cam2"
    fps = 10
    
    [amqp]
    amqp_url = "amqp://user:pass@rabbitmq:5672/meu-cliente"
    EOF
    
    # Executar múltiplas instâncias
    ./edge-video -config cam1.toml &
    ./edge-video -config cam2.toml &
    ```

### Parâmetros de Câmera

| Parâmetro | Tipo | Default | Descrição |
|-----------|------|---------|-----------|
| `rtsp_url` | `string` | - | URL RTSP completa da câmera |
| `camera_id` | `string` | - | Identificador único |
| `fps` | `int` | `5` | Frames por segundo (1-30) |
| `timeout` | `int` | `10` | Timeout de conexão (segundos) |
| `retry_enabled` | `bool` | `true` | Habilita reconexão automática |
| `retry_max_attempts` | `int` | `3` | Máximo de tentativas |
| `retry_delay_seconds` | `int` | `5` | Delay entre tentativas |
| `camera_buffer_size` | `int` | `150` | Frames que podem aguardar por câmera antes de serem descartados |
| `buffer_size` | `int` | `1024` | **Legado**: mantido para compatibilidade, usa `camera_buffer_size` quando ausente |

## Protocolos Suportados

### RTSP (Real-Time Streaming Protocol)

Protocolo padrão para câmeras IP.

**Formato de URL:**
```
rtsp://[user:password@]host[:port]/path
```

**Exemplos:**

```toml
# Hikvision
[camera]
rtsp_url = "rtsp://admin:12345@192.168.1.64:554/Streaming/Channels/101"

# Dahua
[camera]
rtsp_url = "rtsp://admin:admin@192.168.1.108:554/cam/realmonitor?channel=1&subtype=0"

# Intelbras
[camera]
rtsp_url = "rtsp://admin:123456@192.168.1.120:554/cam0_0"

# Axis
[camera]
rtsp_url = "rtsp://root:pass@192.168.1.90/axis-media/media.amp"

# Generic (sem autenticação)
[camera]
rtsp_url = "rtsp://192.168.1.150:554/stream"
```

### HTTP/MJPEG

Suporte a streams MJPEG via HTTP.

```toml
[camera]
http_url = "http://192.168.1.200:8080/video"
camera_id = "webcam1"
```

## Captura e Processamento

### Geração de Timestamp

O Edge Video usa **Unix nanoseconds** (v1.2.0+) para máxima precisão e performance:

```go
// Captura de frame com timestamp
func captureFrame(cameraID string) {
    // Capturar frame da câmera
    frameData := camera.Capture()
    
    // Gerar timestamp Unix nanoseconds
    now := time.Now()
    timestampNano := now.UnixNano()  // int64: 1731073800123456789
    
    // Gerar chave Redis
    sequence := "00001"  // Anti-colisão
    redisKey := fmt.Sprintf("%s:frames:%s:%d:%s", 
        vhost, cameraID, timestampNano, sequence)
    
    // Armazenar no Redis
    redis.Set(redisKey, frameData, ttl)
    
    // Publicar metadata
    metadata := Metadata{
        CameraID:      cameraID,
        Timestamp:     now.Format(time.RFC3339Nano),
        TimestampNano: timestampNano,
        Sequence:      sequence,
        RedisKey:      redisKey,
        Vhost:         vhost,
    }
    
    rabbitmq.Publish("camera.metadata", metadata)
}
```

### Formato do Frame Capturado

**Frame no Redis:**
- **Chave**: `{vhost}:frames:{camera}:{unix_nano}:{seq}`
- **Valor**: Bytes do frame (JPEG/PNG comprimido)
- **TTL**: Configurável (padrão 300s = 5min)

**Metadata no RabbitMQ:**
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

!!! tip "Performance"
    Unix nanoseconds oferece:
    - ✅ **36% menor** que RFC3339 (19 vs 30 caracteres)
    - ✅ **10x mais rápido** para comparações (integer vs string)
    - ✅ Ordenação natural sem parsing

## Frame Rate (FPS)

### Configuração de FPS

```toml
[camera]
fps = 5  # 5 frames por segundo
```

**Recomendações por Caso de Uso:**

| Caso de Uso | FPS | Motivo |
|-------------|-----|--------|
| Vigilância geral | 5-10 | Economia de banda e storage |
| Reconhecimento facial | 15-20 | Captura de detalhes |
| Contagem de pessoas | 10-15 | Tracking adequado |
| Análise de tráfego | 5-10 | Suficiente para fluxo |
| Eventos críticos | 20-30 | Máxima precisão |
| Teste/Debug | 1-2 | Economia para desenvolvimento |

### Cálculo de Recursos

```python
# Estimativa de storage necessário
frames_per_second = 5
frame_size_kb = 250  # Média
ttl_seconds = 300
cameras = 10

# Storage por câmera
storage_per_camera_mb = (frames_per_second * frame_size_kb * ttl_seconds) / 1024
# 5 * 250 * 300 / 1024 = 366 MB

# Storage total
total_storage_mb = storage_per_camera_mb * cameras
# 366 * 10 = 3.66 GB

print(f"Storage necessário: {total_storage_mb:.2f} MB")
print(f"Por câmera: {storage_per_camera_mb:.2f} MB")
```

## Reconexão Automática

### Retry Logic

O Edge Video implementa retry automático para lidar com falhas temporárias:

```go
type RetryConfig struct {
    Enabled     bool
    MaxAttempts int
    Delay       time.Duration
    BackoffRate float64
}

func connectWithRetry(rtspURL string, config RetryConfig) error {
    var err error
    
    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        // Tentar conectar
        conn, err := rtsp.Connect(rtspURL)
        if err == nil {
            log.Printf("Conectado com sucesso: %s", rtspURL)
            return nil
        }
        
        if attempt < config.MaxAttempts {
            delay := time.Duration(float64(config.Delay) * 
                math.Pow(config.BackoffRate, float64(attempt-1)))
            
            log.Printf("Tentativa %d/%d falhou: %v. Retry em %v...", 
                attempt, config.MaxAttempts, err, delay)
            
            time.Sleep(delay)
        }
    }
    
    return fmt.Errorf("falha após %d tentativas: %w", 
        config.MaxAttempts, err)
}
```

### Configuração de Retry

```toml
[camera]
retry_enabled = true
retry_max_attempts = 5
retry_delay_seconds = 5
retry_backoff_rate = 2.0  # Exponential backoff
```

**Comportamento:**
- Tentativa 1: Imediato
- Tentativa 2: Após 5s
- Tentativa 3: Após 10s (5 * 2^1)
- Tentativa 4: Após 20s (5 * 2^2)
- Tentativa 5: Após 40s (5 * 2^3)

## Troubleshooting

### Erro: Connection Timeout

```bash
# Testar conectividade
ping 192.168.1.100

# Testar porta RTSP
telnet 192.168.1.100 554

# Testar com FFmpeg
ffmpeg -rtsp_transport tcp -i "rtsp://admin:pass@192.168.1.100:554/stream1" -frames:v 1 test.jpg
```

**Soluções:**
1. Verificar firewall
2. Confirmar credenciais
3. Testar diferentes portas
4. Usar `rtsp_transport tcp` se UDP falhar

### Erro: Authentication Failed

```toml
# Verificar formato de URL
[camera]
rtsp_url = "rtsp://usuario:senha@ip:porta/caminho"

# Escapar caracteres especiais na senha
# Se senha contém @, use %40
rtsp_url = "rtsp://admin:p%40ssword@192.168.1.100:554/stream1"
```

### Erro: Frame Rate Baixo

```bash
# Verificar CPU
top

# Verificar rede
iftop

# Verificar logs
docker logs camera-collector
```

**Soluções:**
1. Reduzir FPS configurado
2. Aumentar recursos de CPU
3. Verificar largura de banda
4. Reduzir resolução na câmera

### Stream Trava/Congela

```toml
# Aumentar buffer
[camera]
buffer_size = 2048  # Default: 1024

# Reduzir timeout
timeout = 5

# Habilitar TCP transport
rtsp_transport = "tcp"  # Ao invés de UDP
```

## Monitoramento

### Métricas de Captura

```bash
# Verificar logs de captura
docker logs -f camera-collector-cam1

# Estatísticas de frames no Redis
redis-cli KEYS "meu-cliente:frames:cam1:*" | wc -l

# Taxa de captura (frames/min)
watch -n 60 'redis-cli KEYS "meu-cliente:frames:cam1:*" | wc -l'
```

### Health Check

```bash
#!/bin/bash
# health-check.sh

CAMERA_ID="cam1"
VHOST="meu-cliente"

# Verificar frames recentes (últimos 30s)
now=$(date +%s)
threshold=$((now - 30))

frames=$(redis-cli KEYS "${VHOST}:frames:${CAMERA_ID}:*" | \
  while read key; do
    timestamp=$(echo $key | cut -d: -f4)
    timestamp_sec=$((timestamp / 1000000000))
    if [ $timestamp_sec -gt $threshold ]; then
      echo $key
    fi
  done | wc -l)

if [ $frames -gt 0 ]; then
  echo "✅ Câmera $CAMERA_ID está capturando ($frames frames nos últimos 30s)"
  exit 0
else
  echo "❌ Câmera $CAMERA_ID sem frames nos últimos 30s"
  exit 1
fi
```

### Dashboard de Status

```python
import redis
import time
from datetime import datetime

r = redis.Redis(host='localhost', port=6379)

def get_camera_status(vhost, camera_ids):
    """Obtém status de captura de múltiplas câmeras."""
    now = int(time.time())
    threshold = now - 60  # Últimos 60 segundos
    
    status = {}
    
    for cam_id in camera_ids:
        pattern = f"{vhost}:frames:{cam_id}:*"
        recent_frames = 0
        
        for key in r.scan_iter(match=pattern):
            key_str = key.decode()
            timestamp_nano = int(key_str.split(':')[3])
            timestamp = timestamp_nano // 1_000_000_000
            
            if timestamp >= threshold:
                recent_frames += 1
        
        status[cam_id] = {
            'active': recent_frames > 0,
            'frames_last_minute': recent_frames,
            'last_check': datetime.now().isoformat()
        }
    
    return status

# Monitorar continuamente
cameras = ['cam1', 'cam2', 'cam3', 'cam4']

while True:
    print("\n" + "="*60)
    print(f"Status das Câmeras - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*60)
    
    status = get_camera_status('meu-cliente', cameras)
    
    for cam_id, info in status.items():
        icon = "✅" if info['active'] else "❌"
        print(f"{icon} {cam_id}: {info['frames_last_minute']} frames/min")
    
    time.sleep(30)  # Atualizar a cada 30 segundos
```

## Best Practices

!!! success "Recomendações"
    
    1. **Camera ID Único**: Use IDs descritivos (`entrada-principal`, `caixa-1`)
    2. **FPS Otimizado**: Configure FPS baseado no caso de uso (não maximize desnecessariamente)
    3. **TTL Adequado**: Use TTL curto para cache (5-10 min), longo para buffer (1h)
    4. **Retry Habilitado**: Sempre habilite retry para produção
    5. **TCP Transport**: Use TCP ao invés de UDP para redes instáveis
    6. **Monitoramento**: Implemente health checks e alertas
    7. **Credenciais Seguras**: Use variáveis de ambiente, não hardcode senhas
    8. **Buffer Sizing**: Ajuste buffer baseado na largura de banda
    9. **Multi-Instance**: Uma instância por câmera para isolamento
    10. **Logging**: Configure logs estruturados para debugging

## Casos de Uso

### 1. Vigilância 24/7

```toml
[camera]
rtsp_url = "rtsp://admin:pass@192.168.1.100:554/stream1"
camera_id = "entrada-principal"
fps = 5  # Economia de recursos
retry_enabled = true
retry_max_attempts = 10  # Retry agressivo

[redis]
enabled = true
ttl_seconds = 600  # 10 minutos de buffer

[metadata]
enabled = true
```

### 2. Reconhecimento Facial

```toml
[camera]
camera_id = "facial-recognition"
fps = 20  # Alta taxa para captura de rostos
rtsp_url = "rtsp://admin:pass@192.168.1.101:554/stream1"

[redis]
ttl_seconds = 300  # Cache rápido

[metadata]
enabled = true  # Processar cada frame
```

### 3. Análise de Tráfego

```toml
[camera]
camera_id = "trafego-av-principal"
fps = 10
rtsp_url = "rtsp://admin:pass@192.168.1.102:554/stream1"

[redis]
enabled = true
ttl_seconds = 1800  # 30 min para análise posterior

[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/analytics"
```

## Integração Completa

```mermaid
graph TB
    A[Câmera IP] -->|RTSP| B[Camera Collector]
    B -->|Store Frame| C[Redis]
    B -->|Publish Metadata| D[RabbitMQ]
    
    D -->|Subscribe| E[AI Consumer]
    D -->|Subscribe| F[Storage Consumer]
    D -->|Subscribe| G[Analytics Consumer]
    
    E -->|Fetch Frame| C
    F -->|Fetch Frame| C
    G -->|Fetch Frame| C
    
    E -->|Results| H[ML Database]
    F -->|Archive| I[S3/MinIO]
    G -->|Metrics| J[Time-Series DB]
```

Para mais detalhes sobre integração:

<div class="grid cards" markdown>

-   :material-database:{ .lg } __Redis Storage__
    
    Cache de frames com Unix nanoseconds
    
    [:octicons-arrow-right-24: Redis Guide](redis-storage.md)

-   :material-message:{ .lg } __Message Queue__
    
    Distribuição via RabbitMQ
    
    [:octicons-arrow-right-24: RabbitMQ Guide](message-queue.md)

-   :material-file-document:{ .lg } __Metadata Publisher__
    
    Eventos de captura em tempo real
    
    [:octicons-arrow-right-24: Metadata Guide](metadata.md)

</div>
