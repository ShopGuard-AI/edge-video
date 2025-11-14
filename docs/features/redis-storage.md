# Redis Storage

O **Edge Video** utiliza Redis como cache temporário para armazenamento de frames de vídeo, permitindo acesso rápido e distribuído aos dados capturados.

## Visão Geral

O Redis Storage oferece:

- ✅ **Cache de Frames**: Armazenamento temporário com TTL configurável
- ✅ **Multi-Tenant**: Isolamento por vhost do RabbitMQ
- ✅ **Performance Otimizada**: Formato de chave ultra-eficiente (Unix nanoseconds)
- ✅ **Acesso Rápido**: Latência sub-milissegundo para queries
- ✅ **Autenticação**: Suporte a username/password para ambientes seguros

## Configuração Básica

=== "config.toml"

    ```toml
    [redis]
    enabled = true
    address = "redis:6379"
    username = ""  # Opcional
    password = ""  # Opcional
    ttl_seconds = 300
    prefix = "frames"
    
    [amqp]
    amqp_url = "amqp://user:pass@rabbitmq:5672/client-a"
    ```

=== "Docker Compose"

    ```yaml
    services:
      redis:
        image: redis:7-alpine
        command: redis-server --requirepass mypassword
        ports:
          - "6379:6379"
      
      camera-collector:
        image: t3labs/edge-video:latest
        environment:
          - REDIS_PASSWORD=mypassword
    ```

## Formato de Chaves

### Formato Atual (v1.2.0+)

```redis
{vhost}:{prefix}:{cameraID}:{unix_timestamp_nano}:{sequence}
```

**Exemplo:**
```redis
supermercado_vhost:frames:cam4:1731024000123456789:00001
```

**Componentes:**

| Campo | Descrição | Exemplo |
|-------|-----------|---------|
| `vhost` | Identificador do cliente (extraído do AMQP) | `supermercado_vhost` |
| `prefix` | Prefixo configurável | `frames` |
| `cameraID` | ID da câmera | `cam4` |
| `unix_nano` | Unix timestamp em nanosegundos | `1731024000123456789` |
| `sequence` | Sequência anti-colisão (5 dígitos) | `00001` |

### Comparação de Performance

| Aspecto | RFC3339 (Anterior) | Unix Nano (Atual) | Melhoria |
|---------|-------------------|-------------------|----------|
| **Tamanho** | 30 caracteres | 19 dígitos | ✅ **36% menor** |
| **Tipo** | String | Integer | ✅ Numérico |
| **Comparação** | String parsing | Integer comparison | ✅ **10x mais rápido** |
| **Sortabilidade** | Lexicográfica | Numérica nativa | ✅ **Natural** |
| **Range Query** | Complexo | Simples (`>= X AND <= Y`) | ✅ **Eficiente** |
| **Overhead (1M keys)** | ~30 MB | ~19 MB | ✅ **-11 MB** |

## Operações Redis

### Armazenar Frame

=== "Go"

    ```go
    // Geração automática da chave
    key := keyGen.GenerateKey("cam1", time.Now())
    // Resultado: "client-a:frames:cam1:1731024000123456789:00001"

    // Armazenamento com TTL
    err := redisClient.Set(ctx, key, frameData, 5*time.Minute).Err()
    if err != nil {
        log.Printf("Erro ao armazenar frame: %v", err)
    }
    ```

=== "Python"

    ```python
    import redis
    import time
    
    r = redis.Redis(host='localhost', port=6379, decode_responses=False)
    
    # Gerar chave manualmente
    unix_nano = int(time.time() * 1_000_000_000)
    key = f"client-a:frames:cam1:{unix_nano}:00001"
    
    # Armazenar com TTL de 5 minutos
    r.setex(key, 300, frame_data)
    ```

=== "Redis CLI"

    ```bash
    # Armazenar frame com TTL
    redis-cli SETEX "client-a:frames:cam1:1731024000123456789:00001" 300 "<frame_data>"
    
    # Verificar chave foi criada
    redis-cli EXISTS "client-a:frames:cam1:1731024000123456789:00001"
    # Resultado: 1 (existe)
    ```

### Buscar Frame Específico

=== "Bash"

    ```bash
    # Buscar chave exata
    redis-cli GET "supermercado_vhost:frames:cam4:1731024000123456789:00001"

    # Buscar todas as chaves de um cliente
    redis-cli KEYS "supermercado_vhost:frames:*"

    # Buscar frames de uma câmera específica
    redis-cli KEYS "supermercado_vhost:frames:cam4:*"
    ```

=== "Python"

    ```python
    import redis
    
    r = redis.Redis(host='localhost', port=6379, decode_responses=False)
    
    # Buscar chave exata
    frame_data = r.get("supermercado_vhost:frames:cam4:1731024000123456789:00001")
    
    # Buscar todas as chaves de um cliente
    all_keys = r.keys("supermercado_vhost:frames:*")
    print(f"Total de frames: {len(all_keys)}")
    
    # Buscar frames de uma câmera
    cam4_keys = r.keys("supermercado_vhost:frames:cam4:*")
    
    # Obter múltiplos frames
    if cam4_keys:
        frames = r.mget(cam4_keys[:10])  # Primeiros 10 frames
    ```

=== "Go"

    ```go
    // Buscar chave exata
    frameData, err := redisClient.Get(ctx, 
        "supermercado_vhost:frames:cam4:1731024000123456789:00001").Bytes()
    
    // Buscar todas as chaves de um cliente
    keys, err := redisClient.Keys(ctx, "supermercado_vhost:frames:*").Result()
    
    // Buscar frames de uma câmera
    cam4Keys, err := redisClient.Keys(ctx, 
        "supermercado_vhost:frames:cam4:*").Result()
    
    // Obter múltiplos frames
    frames, err := redisClient.MGet(ctx, cam4Keys[:10]...).Result()
    ```

### Range Queries (Janela Temporal)

```bash
# Buscar frames entre timestamps
redis-cli --scan --pattern "client-a:frames:cam1:*" | \
  awk -F: '{if ($4 >= 1731024000000000000 && $4 <= 1731024100000000000) print}'

# Contar frames em período
redis-cli KEYS "client-a:frames:*:173102*" | wc -l

# Últimos frames (ordenados)
redis-cli KEYS "client-a:frames:cam1:*" | sort -t: -k4 -n | tail -10
```

### Estatísticas

```bash
# Total de chaves por cliente
redis-cli KEYS "supermercado_vhost:*" | wc -l

# Memória usada
redis-cli INFO memory | grep used_memory_human

# TTL médio
redis-cli --scan --pattern "client-a:frames:*" | \
  xargs -n1 redis-cli TTL | \
  awk '{sum+=$1; count++} END {print sum/count}'
```

## Multi-Tenancy

### Isolamento por Vhost

Cada cliente possui namespace isolado baseado no vhost do RabbitMQ:

```toml
# Cliente A
[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/client-a"

# Cliente B
[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/client-b"
```

**Resultado no Redis:**
```redis
client-a:frames:cam1:1731024000123456789:00001
client-b:frames:cam1:1731024000123456789:00001
```

!!! tip "Zero Conflitos"
    Mesmo que dois clientes usem câmeras com IDs idênticos, não há risco de colisão de dados.

### Query por Cliente

```bash
# Listar todos os frames do Cliente A
redis-cli KEYS "client-a:*"

# Estatísticas por cliente
redis-cli --scan --pattern "client-a:*" | wc -l
redis-cli --scan --pattern "client-b:*" | wc -l
```

## Migração {#migracao}

!!! warning "Breaking Change v1.2.0"
    A migração de RFC3339 para Unix nanoseconds é uma **mudança incompatível**.

### Opção 1: FLUSHDB (Recomendado para Dev/Staging)

```bash
# Conectar ao Redis
redis-cli

# Limpar todos os dados
FLUSHDB

# Reiniciar aplicação
docker-compose restart camera-collector
```

### Opção 2: Aguardar TTL (Recomendado para Produção)

```bash
# Verificar TTL configurado
grep ttl_seconds config.toml

# Aguardar tempo do TTL (ex: 300 segundos = 5 minutos)
# Chaves antigas expiram automaticamente
```

### Opção 3: Script de Migração

```python
import redis
import re
from datetime import datetime

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# Padrão antigo: frames:{vhost}:{cam}:{RFC3339}:{seq}
old_pattern = "frames:*:*:*:*"

for old_key in r.scan_iter(match=old_pattern):
    # Parse chave antiga
    parts = old_key.split(':')
    if len(parts) != 5:
        continue
    
    _, vhost, cam, timestamp_rfc, seq = parts
    
    # Converter RFC3339 para Unix nano
    dt = datetime.fromisoformat(timestamp_rfc.replace('Z', '+00:00'))
    unix_nano = int(dt.timestamp() * 1_000_000_000)
    
    # Nova chave: {vhost}:frames:{cam}:{unix_nano}:{seq}
    new_key = f"{vhost}:frames:{cam}:{unix_nano}:{seq}"
    
    # Copiar dados
    ttl = r.ttl(old_key)
    data = r.get(old_key)
    r.setex(new_key, ttl if ttl > 0 else 300, data)
    
    # Deletar chave antiga
    r.delete(old_key)
    print(f"Migrated: {old_key} → {new_key}")

print("Migration complete!")
```

## Monitoramento

### RedisInsight (Recomendado)

```yaml
# docker-compose.yml
services:
  redis-insight:
    image: redis/redisinsight:latest
    ports:
      - "5540:5540"
    volumes:
      - redis-insight-data:/data
```

Acesse: `http://localhost:5540`

### Métricas Importantes

```bash
# Uso de memória
redis-cli INFO memory | grep used_memory_human

# Número total de chaves
redis-cli DBSIZE

# Taxa de hit/miss
redis-cli INFO stats | grep keyspace

# Comandos por segundo
redis-cli INFO stats | grep instantaneous_ops_per_sec

# Conexões ativas
redis-cli INFO clients | grep connected_clients
```

### Alertas Recomendados

| Métrica | Threshold | Ação |
|---------|-----------|------|
| `used_memory` | > 80% | Aumentar RAM ou reduzir TTL |
| `keyspace_misses` | > 20% | Revisar estratégia de cache |
| `connected_clients` | > 1000 | Escalar Redis ou usar pool |
| `expired_keys` | Muito baixo | Verificar TTL |

## Troubleshooting

### Erro: Connection Refused

```bash
# Verificar se Redis está rodando
docker ps | grep redis

# Testar conexão
redis-cli -h localhost -p 6379 PING

# Verificar logs
docker logs redis
```

### Erro: Authentication Failed

```toml
# Adicionar password no config.toml
[redis]
password = "sua_senha_aqui"
```

```bash
# Testar autenticação
redis-cli -a sua_senha_aqui PING
```

### Memória Excessiva

```bash
# Verificar TTL das chaves
redis-cli --scan --pattern "client-a:*" | \
  xargs -n1 redis-cli TTL | sort -n

# Forçar expiração
redis-cli --scan --pattern "client-a:*" | \
  xargs redis-cli DEL
```

### Performance Lenta

```bash
# Verificar comandos lentos
redis-cli SLOWLOG GET 10

# Habilitar logging de comandos lentos
redis-cli CONFIG SET slowlog-log-slower-than 10000

# Analisar comandos
redis-cli --latency
```

## Best Practices

!!! success "Recomendações"
    
    1. **TTL Adequado**: Configure TTL baseado no caso de uso (5-10 minutos para cache)
    2. **Monitoramento**: Use RedisInsight para visualização em tempo real
    3. **Backup**: Configure persistência RDB/AOF para dados críticos
    4. **Segurança**: Sempre use password em produção
    5. **Escalabilidade**: Considere Redis Cluster para alta disponibilidade
    6. **Namespace**: Use vhosts para isolamento multi-tenant
    7. **Limpeza**: Confie no TTL automático ao invés de DEL manual
    8. **KEYS em Produção**: Evite `KEYS *` em produção, use `SCAN` instead
    9. **Pipeline**: Use pipeline para operações batch
    10. **Connection Pool**: Reutilize conexões com pool

## Casos de Uso

### 1. Sistema de Vigilância em Tempo Real

**Cenário:** Monitoramento de 20 câmeras com acesso frequente aos últimos frames.

```toml
[redis]
enabled = true
ttl_seconds = 300  # 5 minutos
prefix = "surveillance"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/security-system"
```

**Queries comuns:**
```bash
# Último frame de cada câmera
for i in {1..20}; do
  redis-cli KEYS "security-system:surveillance:cam$i:*" | sort -t: -k4 -n | tail -1
done

# Frames dos últimos 30 segundos
now=$(date +%s)
start=$((now - 30))
redis-cli --scan --pattern "security-system:surveillance:*" | \
  awk -F: -v s="${start}000000000" '{if ($4 >= s) print}'
```

### 2. Analytics de Tráfego

**Cenário:** Análise de tráfego em loja com 5 câmeras, frames armazenados para processamento.

```toml
[redis]
enabled = true
ttl_seconds = 600  # 10 minutos para análise
prefix = "analytics"

[metadata]
enabled = true  # Metadados para ML
```

**Python Analytics:**
```python
import redis
import time
from collections import defaultdict

r = redis.Redis(host='localhost', port=6379)

def get_frames_last_hour(camera_id):
    """Busca frames da última hora."""
    now = int(time.time())
    one_hour_ago = now - 3600
    
    pattern = f"loja-centro:analytics:{camera_id}:*"
    frames = []
    
    for key in r.scan_iter(match=pattern):
        key_str = key.decode()
        timestamp_nano = int(key_str.split(':')[3])
        timestamp = timestamp_nano // 1_000_000_000
        
        if timestamp >= one_hour_ago:
            frames.append({
                'key': key_str,
                'timestamp': timestamp,
                'data': r.get(key)
            })
    
    return sorted(frames, key=lambda x: x['timestamp'])

def analyze_traffic():
    """Analisa tráfego por câmera."""
    stats = defaultdict(int)
    
    for cam_id in ['entrada', 'caixa1', 'caixa2', 'saida']:
        frames = get_frames_last_hour(cam_id)
        stats[cam_id] = len(frames)
        print(f"{cam_id}: {len(frames)} frames na última hora")
    
    return stats

# Executar análise
traffic_stats = analyze_traffic()
```

### 3. Backup Temporário para Falhas de Rede

**Cenário:** Edge device com conectividade intermitente, Redis como buffer local.

```toml
[redis]
enabled = true
ttl_seconds = 3600  # 1 hora de buffer
prefix = "backup"

[amqp]
amqp_url = "amqp://user:pass@cloud-rabbitmq:5672/edge-site-01"
```

**Go Recovery Service:**
```go
package main

import (
    "context"
    "log"
    "github.com/go-redis/redis/v8"
    "github.com/streadway/amqp"
)

func recoverAndPublish(redisClient *redis.Client, amqpChannel *amqp.Channel) {
    ctx := context.Background()
    
    // Buscar frames que não foram publicados
    pattern := "edge-site-01:backup:*"
    iter := redisClient.Scan(ctx, 0, pattern, 100).Iterator()
    
    published := 0
    for iter.Next(ctx) {
        key := iter.Val()
        
        // Obter dados do frame
        frameData, err := redisClient.Get(ctx, key).Bytes()
        if err != nil {
            continue
        }
        
        // Publicar no RabbitMQ
        err = amqpChannel.Publish(
            "cameras",
            "camera.backup",
            false,
            false,
            amqp.Publishing{
                ContentType: "application/octet-stream",
                Body:        frameData,
            },
        )
        
        if err == nil {
            // Remover do Redis após publicação bem-sucedida
            redisClient.Del(ctx, key)
            published++
        }
    }
    
    log.Printf("Recuperados e publicados %d frames", published)
}
```

### 4. Multi-Tenant SaaS

**Cenário:** Plataforma SaaS com múltiplos clientes, isolamento completo.

```toml
# config-cliente-a.toml
[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/cliente-a"

# config-cliente-b.toml
[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/cliente-b"
```

**Admin Dashboard (Python):**
```python
import redis

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

def get_tenant_stats():
    """Dashboard de estatísticas por cliente."""
    tenants = ['cliente-a', 'cliente-b', 'cliente-c']
    stats = {}
    
    for tenant in tenants:
        pattern = f"{tenant}:frames:*"
        keys = list(r.scan_iter(match=pattern))
        
        if keys:
            # Total de frames
            total_frames = len(keys)
            
            # Memória usada (aproximado)
            sample_key = keys[0]
            sample_size = len(r.get(sample_key) or b'')
            total_memory_mb = (total_frames * sample_size) / (1024 * 1024)
            
            # Câmeras únicas
            cameras = set(k.split(':')[2] for k in keys)
            
            stats[tenant] = {
                'frames': total_frames,
                'memory_mb': round(total_memory_mb, 2),
                'cameras': len(cameras),
                'camera_ids': list(cameras)
            }
        else:
            stats[tenant] = {'frames': 0, 'memory_mb': 0, 'cameras': 0}
    
    return stats

# Exibir estatísticas
for tenant, data in get_tenant_stats().items():
    print(f"\n{tenant.upper()}:")
    print(f"  Frames: {data['frames']}")
    print(f"  Memória: {data['memory_mb']} MB")
    print(f"  Câmeras: {data['cameras']} ({', '.join(data.get('camera_ids', []))})")
```

### 5. Time-Series Analysis

**Cenário:** Análise temporal de frames com agregações.

```python
import redis
import time
from datetime import datetime, timedelta

r = redis.Redis(host='localhost', port=6379)

def get_frames_by_time_window(camera_id, minutes=10):
    """Busca frames em janela temporal."""
    now = int(time.time())
    start = now - (minutes * 60)
    
    pattern = f"*:frames:{camera_id}:*"
    frames_by_minute = {}
    
    for key in r.scan_iter(match=pattern):
        key_str = key.decode()
        timestamp_nano = int(key_str.split(':')[3])
        timestamp = timestamp_nano // 1_000_000_000
        
        if timestamp >= start:
            minute_bucket = timestamp - (timestamp % 60)
            minute_key = datetime.fromtimestamp(minute_bucket).strftime('%Y-%m-%d %H:%M')
            
            if minute_key not in frames_by_minute:
                frames_by_minute[minute_key] = []
            
            frames_by_minute[minute_key].append({
                'key': key_str,
                'timestamp': timestamp,
                'size': r.strlen(key)
            })
    
    return frames_by_minute

def plot_frame_rate():
    """Plota taxa de frames por minuto."""
    data = get_frames_by_time_window('cam1', minutes=10)
    
    for minute, frames in sorted(data.items()):
        count = len(frames)
        avg_size = sum(f['size'] for f in frames) / count if count else 0
        print(f"{minute}: {count} frames (avg {avg_size:.0f} bytes)")

# Executar análise
plot_frame_rate()
```

## Integração com Aplicações

### Consumer Python Completo

```python
import redis
import pika
import json
import base64
from typing import Optional

class FrameConsumer:
    """Consumer que busca frames do Redis ao receber metadados."""
    
    def __init__(self, redis_host='localhost', rabbitmq_host='localhost', 
                 vhost='/', user='guest', password='guest'):
        # Conexão Redis
        self.redis = redis.Redis(host=redis_host, port=6379, decode_responses=False)
        
        # Conexão RabbitMQ
        credentials = pika.PlainCredentials(user, password)
        parameters = pika.ConnectionParameters(
            host=rabbitmq_host,
            virtual_host=vhost,
            credentials=credentials
        )
        self.connection = pika.BlockingConnection(parameters)
        self.channel = self.connection.channel()
        
        self.vhost = vhost.replace('/', '')
    
    def fetch_frame_from_redis(self, camera_id: str, timestamp_nano: int, 
                              sequence: str) -> Optional[bytes]:
        """Busca frame do Redis usando metadados."""
        key = f"{self.vhost}:frames:{camera_id}:{timestamp_nano}:{sequence}"
        return self.redis.get(key)
    
    def callback(self, ch, method, properties, body):
        """Callback para mensagens de metadados."""
        try:
            # Parse metadata
            metadata = json.loads(body)
            
            camera_id = metadata['camera_id']
            timestamp_nano = metadata['timestamp_nano']
            sequence = metadata.get('sequence', '00001')
            
            # Buscar frame do Redis
            frame_data = self.fetch_frame_from_redis(
                camera_id, timestamp_nano, sequence
            )
            
            if frame_data:
                print(f"Frame encontrado: {camera_id} - {len(frame_data)} bytes")
                # Processar frame...
            else:
                print(f"Frame não encontrado no Redis: {camera_id}")
        
        except Exception as e:
            print(f"Erro ao processar mensagem: {e}")
    
    def start_consuming(self):
        """Inicia consumo de metadados."""
        self.channel.exchange_declare(exchange='camera.metadata', 
                                     exchange_type='topic')
        
        result = self.channel.queue_declare(queue='', exclusive=True)
        queue_name = result.method.queue
        
        self.channel.queue_bind(exchange='camera.metadata',
                               queue=queue_name,
                               routing_key='camera.metadata.event')
        
        self.channel.basic_consume(queue=queue_name,
                                  on_message_callback=self.callback,
                                  auto_ack=True)
        
        print('Aguardando metadados...')
        self.channel.start_consuming()

# Usar
consumer = FrameConsumer(
    redis_host='localhost',
    rabbitmq_host='localhost',
    vhost='meu-cliente',
    user='user',
    password='password'
)
consumer.start_consuming()
```

## Próximos Passos

<div class="grid cards" markdown>

-   :material-file-document:{ .lg } __Multi-Tenancy__
    
    Entenda como funciona o isolamento por vhost
    
    [:octicons-arrow-right-24: Vhost Guide](../vhost-based-identification.md)

-   :material-chart-line:{ .lg } __Monitoramento__
    
    Configure métricas e alertas
    
    [:octicons-arrow-right-24: Monitoring](../guides/monitoring.md)

-   :material-cog:{ .lg } __Configuração Avançada__
    
    Otimize seu Redis para produção
    
    [:octicons-arrow-right-24: Advanced Config](../guides/advanced-config.md)

</div>

