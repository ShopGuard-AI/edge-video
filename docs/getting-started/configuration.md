# Configuração

Guia completo de configuração do Edge Video usando TOML.

## Arquivo de Configuração

O Edge Video usa **TOML** como formato de configuração. O arquivo padrão é `config.toml` localizado no diretório raiz da aplicação.

!!! tip "Localização do Arquivo"
    Por padrão, a aplicação procura `config.toml` no diretório atual. Você pode especificar outro caminho:
    ```bash
    ./edge-video --config /path/to/custom-config.toml
    ```

## Configuração Mínima

A configuração mínima requer apenas câmeras e conexão AMQP:

```toml
# Configuração AMQP
[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/meu_vhost"
exchange = "cameras"
routing_key_prefix = "camera"

# Pelo menos uma câmera
[[cameras]]
id = "cam1"
url = "rtsp://usuario:senha@192.168.1.10:554/stream"
```

## Estrutura Completa

```toml
# =============================================================================
# CONFIGURAÇÃO GERAL
# =============================================================================

# Intervalo entre capturas em milissegundos (opcional)
# Quanto menor, mais frames capturados (maior carga)
# Padrão: 500ms (2 frames por segundo)
interval_ms = 500

# Protocolo de mensageria: "amqp" ou "mqtt"
# AMQP: RabbitMQ (recomendado)
# MQTT: Mosquitto, HiveMQ, etc
# Padrão: "amqp"
protocol = "amqp"

# Processar a cada N frames (opcional)
# Reduz carga capturando 1 a cada N frames
# Exemplo: 3 = captura 1 frame a cada 3
# Padrão: 1 (todos os frames)
process_every_n_frames = 1

# =============================================================================
# AMQP (RabbitMQ)
# =============================================================================

[amqp]
# URL de conexão AMQP
# Formato: amqp://[user]:[password]@[host]:[port]/[vhost]
# IMPORTANTE: O vhost é usado para isolamento multi-tenant no Redis
amqp_url = "amqp://user:password@rabbitmq:5672/supermercado_vhost"

# Nome do exchange para publicação de frames
exchange = "cameras"

# Prefixo para routing keys
# Routing key final: {routing_key_prefix}.{camera_id}
# Exemplo: "camera.cam1"
routing_key_prefix = "camera"

# =============================================================================
# MQTT (Alternativa ao AMQP)
# =============================================================================

[mqtt]
# URL do broker MQTT
# Formato: tcp://[host]:[port] ou ssl://[host]:[port]
broker = "tcp://localhost:1883"

# Prefixo para tópicos MQTT
# Tópico final: {topic_prefix}{camera_id}
# Exemplo: "camera/cam1"
topic_prefix = "camera/"

# Usuário MQTT (opcional)
username = ""

# Senha MQTT (opcional)
password = ""

# Client ID (opcional, gerado automaticamente se vazio)
client_id = ""

# =============================================================================
# REDIS STORAGE
# =============================================================================

[redis]
# Habilitar armazenamento Redis
# Se false, frames só são publicados no message broker
enabled = true

# Endereço do Redis
# Formato: [host]:[port]
address = "redis:6379"

# Usuário Redis (opcional, deixe vazio se não tiver)
username = ""

# Senha Redis (opcional, deixe vazio se não tiver)
password = ""

# TTL (Time To Live) em segundos
# Tempo que os frames ficam armazenados antes de expirar
# Recomendado: 60-600 (1-10 minutos)
ttl_seconds = 300

# Prefixo para chaves Redis
# Chave final: {vhost}:{prefix}:{cameraID}:{unix_nano}:{sequence}
# Exemplo: "supermercado_vhost:frames:cam1:1731024000123456789:00001"
prefix = "frames"

# Database Redis (0-15)
# Permite separação lógica no mesmo servidor Redis
db = 0

# Timeout de conexão em segundos
connection_timeout = 5

# =============================================================================
# METADATA PUBLISHER
# =============================================================================

[metadata]
# Habilitar publicação de metadados
# Publica informações sobre frames (timestamp, tamanho, etc)
enabled = true

# Exchange para metadados (separado dos frames)
exchange = "camera.metadata"

# Routing key para eventos de metadados
routing_key = "camera.metadata.event"

# =============================================================================
# COMPRESSÃO
# =============================================================================

[compression]
# Habilitar compressão Zstd nos frames
# Reduz uso de rede mas aumenta uso de CPU
enabled = false

# Nível de compressão Zstd (1-22)
# 1: Mais rápido, menor compressão
# 22: Mais lento, maior compressão
# Recomendado: 3-5 para produção
level = 3

# =============================================================================
# CÂMERAS RTSP
# =============================================================================

# Adicione quantas câmeras precisar
# Cada câmera é processada em paralelo

[[cameras]]
id = "cam1"
url = "rtsp://admin:password@192.168.1.10:554/stream1"

[[cameras]]
id = "cam2"
url = "rtsp://admin:password@192.168.1.11:554/stream1"

[[cameras]]
id = "cam3"
url = "rtsp://admin:password@192.168.1.12:554/stream1"

# Exemplo com câmera pública de teste
[[cameras]]
id = "test"
url = "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mp4"
```

## Parâmetros Detalhados

### Geral

#### `interval_ms`

**Tipo:** Integer  
**Padrão:** 500  
**Descrição:** Intervalo em milissegundos entre capturas de frames.

**Exemplos:**
```toml
interval_ms = 100   # 10 FPS (alta taxa)
interval_ms = 500   # 2 FPS (padrão)
interval_ms = 1000  # 1 FPS (baixa taxa)
```

**Considerações:**
- Valores menores = mais frames = maior carga CPU/rede
- Valores maiores = menos frames = menor carga
- Recomendado: 100-1000ms dependendo do caso de uso

#### `protocol`

**Tipo:** String  
**Padrão:** "amqp"  
**Valores:** "amqp", "mqtt"  
**Descrição:** Protocolo de mensageria para publicação de frames.

```toml
protocol = "amqp"  # RabbitMQ (recomendado)
protocol = "mqtt"  # MQTT brokers
```

#### `process_every_n_frames`

**Tipo:** Integer  
**Padrão:** 1  
**Descrição:** Processa 1 a cada N frames capturados.

```toml
process_every_n_frames = 1  # Todos os frames
process_every_n_frames = 3  # 1 a cada 3 frames
process_every_n_frames = 5  # 1 a cada 5 frames
```

**Uso:** Reduzir carga mantendo captura frequente.

### AMQP (RabbitMQ)

#### `amqp_url`

**Tipo:** String  
**Obrigatório:** Sim  
**Formato:** `amqp://[user]:[pass]@[host]:[port]/[vhost]`

```toml
# Desenvolvimento
amqp_url = "amqp://guest:guest@localhost:5672/"

# Produção com vhost dedicado
amqp_url = "amqp://user:password@rabbitmq.prod.com:5672/cliente-a"

# Com SSL/TLS
amqp_url = "amqps://user:password@rabbitmq.prod.com:5671/cliente-a"
```

!!! warning "Vhost e Multi-Tenancy"
    O **vhost** extraído da URL é usado para isolar dados no Redis:
    
    - `amqp://user:pass@host/client-a` → Redis keys: `client-a:frames:*`
    - `amqp://user:pass@host/client-b` → Redis keys: `client-b:frames:*`
    
    [Saiba mais sobre Multi-Tenancy](../vhost-based-identification.md)

#### `exchange`

**Tipo:** String  
**Padrão:** "cameras"  
**Descrição:** Nome do exchange RabbitMQ para publicação.

```toml
exchange = "cameras"              # Simples
exchange = "supermercado.cameras" # Namespaced
```

#### `routing_key_prefix`

**Tipo:** String  
**Padrão:** "camera"  
**Descrição:** Prefixo para routing keys.

```toml
routing_key_prefix = "camera"  # → camera.cam1, camera.cam2
routing_key_prefix = "video"   # → video.cam1, video.cam2
```

### Redis Storage

#### `enabled`

**Tipo:** Boolean  
**Padrão:** true  
**Descrição:** Habilita/desabilita storage no Redis.

```toml
[redis]
enabled = true   # Frames armazenados no Redis + publicados
enabled = false  # Apenas publicados no message broker
```

#### `address`

**Tipo:** String  
**Padrão:** "localhost:6379"  
**Formato:** `[host]:[port]`

```toml
address = "redis:6379"           # Docker Compose
address = "localhost:6379"       # Local
address = "10.0.1.50:6379"       # IP remoto
address = "redis.prod.com:6379"  # DNS
```

#### `password`

**Tipo:** String  
**Padrão:** ""  
**Descrição:** Senha para autenticação Redis.

```toml
password = ""              # Sem autenticação (dev)
password = "secret123"     # Com autenticação (prod)
```

#### `ttl_seconds`

**Tipo:** Integer  
**Padrão:** 300  
**Descrição:** Tempo de vida dos frames em segundos.

```toml
ttl_seconds = 60    # 1 minuto (cache curto)
ttl_seconds = 300   # 5 minutos (padrão)
ttl_seconds = 600   # 10 minutos (cache longo)
ttl_seconds = 3600  # 1 hora (muito tempo)
```

**Considerações:**
- TTL curto = menos memória Redis
- TTL longo = mais histórico disponível
- Recomendado: 60-600s dependendo do caso

#### `prefix`

**Tipo:** String  
**Padrão:** "frames"  
**Descrição:** Prefixo para chaves Redis.

```toml
prefix = "frames"   # → client-a:frames:cam1:...
prefix = "video"    # → client-a:video:cam1:...
prefix = "capture"  # → client-a:capture:cam1:...
```

!!! info "Formato de Chave Completa v1.2.0+"
    ```
    {vhost}:{prefix}:{cameraID}:{unix_timestamp_nano}:{sequence}
    ```
    
    Exemplo:
    ```
    supermercado_vhost:frames:cam4:1731024000123456789:00001
    ```
    
    [Documentação completa do formato](../features/redis-storage.md)

#### `db`

**Tipo:** Integer  
**Padrão:** 0  
**Range:** 0-15  
**Descrição:** Database Redis (separação lógica).

```toml
db = 0  # Database padrão
db = 1  # Database alternativa
```

### Metadata Publisher

#### `enabled`

**Tipo:** Boolean  
**Padrão:** true  
**Descrição:** Publica metadados de frames separadamente.

```toml
[metadata]
enabled = true   # Publica frames + metadados
enabled = false  # Apenas frames
```

**Metadados publicados:**
- Timestamp de captura
- Camera ID
- Tamanho do frame (bytes)
- Chave Redis (se storage habilitado)
- Vhost do cliente

#### `exchange` e `routing_key`

**Tipo:** String  
**Descrição:** Destino das mensagens de metadados.

```toml
exchange = "camera.metadata"
routing_key = "camera.metadata.event"
```

### Compressão

#### `enabled`

**Tipo:** Boolean  
**Padrão:** false  
**Descrição:** Habilita compressão Zstd.

```toml
[compression]
enabled = false  # Sem compressão (mais CPU, menos rede)
enabled = true   # Com compressão (menos CPU, mais rede)
```

**Trade-offs:**
- Habilitado: -50% tráfego de rede, +20% CPU
- Desabilitado: Mais simples, menos overhead

#### `level`

**Tipo:** Integer  
**Padrão:** 3  
**Range:** 1-22  
**Descrição:** Nível de compressão Zstd.

```toml
level = 1   # Rápido, compressão baixa
level = 3   # Balanceado (recomendado)
level = 5   # Mais compressão, mais CPU
level = 10  # Alta compressão, muito CPU
level = 22  # Máxima compressão (não recomendado)
```

### Câmeras

#### Formato

```toml
[[cameras]]
id = "identificador_unico"
url = "rtsp://usuario:senha@host:porta/caminho"
```

#### `id`

**Tipo:** String  
**Obrigatório:** Sim  
**Descrição:** Identificador único da câmera.

```toml
[[cameras]]
id = "cam1"           # Simples
id = "entrada-principal"  # Descritivo
id = "loja-01-caixa-02"  # Hierárquico
```

**Restrições:**
- Deve ser único por configuração
- Sem espaços (use `-` ou `_`)
- Alfanumérico recomendado

#### `url`

**Tipo:** String  
**Obrigatório:** Sim  
**Formato:** RTSP URL  
**Descrição:** URL da stream RTSP.

```toml
# Formato completo
url = "rtsp://usuario:senha@192.168.1.10:554/stream1"

# Sem autenticação
url = "rtsp://192.168.1.10:554/stream"

# Porta não-padrão
url = "rtsp://admin:pass@camera.local:8554/live"

# Stream pública de teste
url = "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mp4"
```

## Exemplos de Configuração

### Desenvolvimento Local

```toml
interval_ms = 1000  # 1 FPS (leve)
protocol = "amqp"

[amqp]
amqp_url = "amqp://guest:guest@localhost:5672/"
exchange = "cameras"
routing_key_prefix = "camera"

[redis]
enabled = true
address = "localhost:6379"
password = ""
ttl_seconds = 60  # TTL curto para dev

[metadata]
enabled = false  # Desabilitado para simplicidade

[compression]
enabled = false

[[cameras]]
id = "test1"
url = "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mp4"
```

### Produção Multi-Tenant

```toml
interval_ms = 500
protocol = "amqp"

[amqp]
# Vhost dedicado por cliente
amqp_url = "amqp://edgevideo:SecurePass123@rabbitmq.prod.com:5672/supermercado-abc"
exchange = "supermercado.cameras"
routing_key_prefix = "camera"

[redis]
enabled = true
address = "redis-cluster.prod.com:6379"
password = "RedisSecurePass456"
ttl_seconds = 300
prefix = "frames"

[metadata]
enabled = true
exchange = "supermercado.metadata"
routing_key = "supermercado.metadata.event"

[compression]
enabled = true
level = 5

# Múltiplas câmeras
[[cameras]]
id = "entrada"
url = "rtsp://admin:CamPass1@10.0.1.10:554/stream1"

[[cameras]]
id = "caixa-01"
url = "rtsp://admin:CamPass1@10.0.1.11:554/stream1"

[[cameras]]
id = "estoque"
url = "rtsp://admin:CamPass1@10.0.1.12:554/stream1"
```

### Alta Performance (50+ câmeras)

```toml
interval_ms = 200  # 5 FPS
process_every_n_frames = 2  # Captura 1 a cada 2
protocol = "amqp"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq-cluster.prod.com:5672/megastore"
exchange = "cameras"
routing_key_prefix = "cam"

[redis]
enabled = true
address = "redis-cluster.prod.com:6379"
password = "secure_password"
ttl_seconds = 120  # TTL curto para economizar memória
prefix = "frames"

[metadata]
enabled = true
exchange = "camera.metadata"
routing_key = "metadata.event"

[compression]
enabled = true
level = 3  # Balanceado

# 50 câmeras...
[[cameras]]
id = "cam01"
url = "rtsp://admin:pass@10.0.1.10:554/stream1"

[[cameras]]
id = "cam02"
url = "rtsp://admin:pass@10.0.1.11:554/stream1"

# ... mais 48 câmeras
```

## Variáveis de Ambiente

Você pode sobrescrever configurações usando variáveis de ambiente:

```bash
# AMQP
export AMQP_URL="amqp://user:pass@rabbitmq:5672/vhost"
export AMQP_EXCHANGE="cameras"

# Redis
export REDIS_ADDRESS="redis:6379"
export REDIS_PASSWORD="secret"
export REDIS_TTL="300"

# Geral
export INTERVAL_MS="500"
export PROTOCOL="amqp"

# Executar
./edge-video
```

## Validação de Configuração

Valide seu arquivo antes de executar:

```bash
# Usando ferramenta de validação
./edge-video validate --config config.toml

# Teste de conexão
./edge-video test-connections --config config.toml

# Dry-run (não captura, apenas valida)
./edge-video --config config.toml --dry-run
```

## Troubleshooting

### Erro: Cannot connect to RabbitMQ

```toml
# Verifique:
[amqp]
amqp_url = "amqp://user:password@correct-host:5672/vhost"
#           ^       ^        ^           ^         ^      ^
#        protocolo user  senha        host      porta  vhost
```

### Erro: Redis authentication failed

```toml
[redis]
password = "correct_password"  # Adicione a senha correta
```

### Erro: Camera RTSP timeout

```toml
# Teste a URL manualmente:
# ffmpeg -i "rtsp://url" -frames:v 1 output.jpg

[[cameras]]
url = "rtsp://correct-url"  # Verifique IP, porta, caminho
```

### Performance ruim com muitas câmeras

```toml
# Reduza carga
interval_ms = 1000           # Menos frames
process_every_n_frames = 3   # Skip frames

[redis]
ttl_seconds = 60             # TTL menor

[compression]
enabled = true               # Ative compressão
level = 3
```

## Próximos Passos

<div class="grid cards" markdown>

-   :material-rocket-launch:{ .lg } __Quick Start__
    
    Comece a usar agora
    
    [:octicons-arrow-right-24: Quick Start](quickstart.md)

-   :material-memory:{ .lg } __Redis Storage__
    
    Entenda o formato de chaves otimizado
    
    [:octicons-arrow-right-24: Redis Guide](../features/redis-storage.md)

-   :material-domain:{ .lg } __Multi-Tenancy__
    
    Configure isolamento por cliente
    
    [:octicons-arrow-right-24: Vhost Guide](../vhost-based-identification.md)

-   :material-docker:{ .lg } __Deploy__
    
    Coloque em produção
    
    [:octicons-arrow-right-24: Docker Guide](../guides/docker.md)

</div>

[← Instalação](installation.md){ .md-button }
[Quick Start →](quickstart.md){ .md-button .md-button--primary }

