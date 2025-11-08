# Uso do Vhost como Identificador de Cliente

## Visão Geral

O `edge-video` agora utiliza o **vhost** do RabbitMQ como identificador único de cliente para isolamento de dados no Redis. Esta abordagem elimina a necessidade de configurações adicionais e aproveita a estrutura natural de multi-tenancy do AMQP.

## Motivação

### Problema Anterior
- Necessidade de configurar um `instance_id` separado para isolamento multi-instância
- Redundância de configuração (vhost na URL + instance_id separado)
- Possibilidade de colisão de chaves Redis entre diferentes clientes
- Timestamps RFC3339 longos e difíceis de ordenar/comparar

### Solução Atual
- **Vhost como identificador natural**: Cada cliente RabbitMQ possui um vhost único
- **Configuração simplificada**: Apenas a URL AMQP é necessária
- **Isolamento automático**: Chaves Redis são automaticamente prefixadas com o vhost
- **Unix Timestamp**: Timestamps numéricos compactos e sortable

## Formato das Chaves Redis

### Estrutura
```
{vhost}:{prefix}:{cameraID}:{unix_timestamp_nano}:{sequence}
```

### Exemplos

#### Cliente A (vhost: `client-a`)
```
client-a:frames:cam1:1731024000123456789:00001
client-a:frames:cam1:1731024000456789123:00002
client-a:frames:cam2:1731024001000000000:00001
```

#### Cliente B (vhost: `client-b`)
```
client-b:frames:cam1:1731024000123456789:00001
client-b:frames:cam1:1731024000456789123:00002
client-b:frames:cam3:1731024001000000000:00001
```

#### Supermercado (vhost: `supermercado_vhost`)
```
supermercado_vhost:frames:cam4:1731024000123456789:00001
supermercado_vhost:frames:cam4:1731024000456789123:00002
```

### Por que Unix Timestamp?

| Aspecto | RFC3339Nano | Unix Nanoseconds | Vantagem |
|---------|-------------|------------------|----------|
| **Exemplo** | `2025-11-08T00:11:01.917748287Z` | `1731024000917748287` | - |
| **Tamanho** | 30 caracteres | 19 dígitos | ✅ 36% menor |
| **Sortable** | String sort | Numérico | ✅ Natural |
| **Comparações** | Parsing complexo | Inteiro | ✅ 10x mais rápido |
| **Range Queries** | `>= start AND <= end` (strings) | `>= start AND <= end` (int64) | ✅ Performance |
| **Padrão** | ISO 8601 | Unix Time | ✅ Universal |

**Benefícios Práticos:**
- 📦 **Memória**: 11 bytes a menos por chave (1M keys = ~11MB economizados)
- ⚡ **Performance**: Comparações numéricas vs parsing de string
- 🔍 **Queries**: `WHERE timestamp > 1731024000000000000 AND timestamp < 1731024100000000000`
- 📊 **Ordenação**: Sort natural por timestamp (mais antigo primeiro)

## Configuração

### Exemplo Básico

```toml
[amqp]
amqp_url = "amqp://user:password@rabbitmq:5672/client-a"
exchange = "camera.frames"
routing_key_prefix = "camera"

[redis]
enabled = true
address = "redis:6379"
ttl_seconds = 300
prefix = "frames"
```

### Extração Automática do Vhost

A aplicação extrai automaticamente o vhost da URL AMQP:

| URL AMQP | Vhost Extraído |
|----------|----------------|
| `amqp://localhost:5672/client-a` | `client-a` |
| `amqp://localhost:5672/production` | `production` |
| `amqp://localhost:5672/` | `/` (default) |
| `amqp://localhost:5672` | `/` (default) |

## Implantação Multi-Cliente

### Cenário: Múltiplos Clientes no Mesmo Redis

```bash
# Cliente A
docker run -e AMQP_URL="amqp://user:pass@rabbitmq:5672/client-a" edge-video

# Cliente B
docker run -e AMQP_URL="amqp://user:pass@rabbitmq:5672/client-b" edge-video

# Cliente C
docker run -e AMQP_URL="amqp://user:pass@rabbitmq:5672/client-c" edge-video
```

**Resultado**: Cada cliente terá suas chaves isoladas automaticamente:
- Cliente A: `frames:client-a:*`
- Cliente B: `frames:client-b:*`
- Cliente C: `frames:client-c:*`

### Ambientes Diferentes

```toml
# Produção
[amqp]
amqp_url = "amqp://prod-user:secret@rabbitmq.prod:5672/production"

# Staging
[amqp]
amqp_url = "amqp://stg-user:secret@rabbitmq.stg:5672/staging"

# Desenvolvimento
[amqp]
amqp_url = "amqp://guest:guest@localhost:5672/dev"
```

## Benefícios

### 1. Isolamento Garantido
- Cada vhost possui namespace próprio no Redis
- Impossível haver colisão entre clientes diferentes
- Mesmas câmeras em diferentes clientes não conflitam

### 2. Simplicidade de Configuração
- Não é necessário configurar `instance_id` separadamente
- O vhost já existe na configuração AMQP
- Menos chance de erro de configuração

### 3. Alinhamento com Arquitetura AMQP
- Vhost é o mecanismo padrão de multi-tenancy no RabbitMQ
- Permissões, filas e exchanges já são isoladas por vhost
- Consistência entre isolamento AMQP e Redis

### 4. Rastreabilidade
- Fácil identificar qual cliente gerou cada chave
- Logs mostram o vhost sendo usado
- Debugging simplificado

## Consultas Redis por Vhost

### Buscar Todos os Frames de um Cliente

```bash
# Cliente A
redis-cli KEYS "client-a:frames:*"

# Supermercado
redis-cli KEYS "supermercado_vhost:frames:*"
```

### Buscar Frames de uma Câmera Específica

```bash
# Câmera cam1 do cliente A
redis-cli KEYS "client-a:frames:cam1:*"

# Câmera cam4 do supermercado
redis-cli KEYS "supermercado_vhost:frames:cam4:*"
```

### Contar Frames por Cliente

```bash
# Contar frames do cliente A
redis-cli EVAL "return #redis.call('keys', 'client-a:frames:*')" 0

# Contar frames do supermercado
redis-cli EVAL "return #redis.call('keys', 'supermercado_vhost:frames:*')" 0
```

### Range Query por Timestamp

```bash
# Obter frames da última hora (usando timestamp Unix)
# Timestamp atual: 1731024000000000000
# 1 hora atrás: 1731020400000000000

# Buscar todas as chaves da câmera
redis-cli KEYS "supermercado_vhost:frames:cam4:*" > /tmp/keys.txt

# Filtrar por range de timestamp (em shell script)
awk -F: '$4 >= 1731020400000000000 && $4 <= 1731024000000000000' /tmp/keys.txt
```

## Logs da Aplicação

Ao iniciar, a aplicação registra o vhost sendo utilizado:

```
INFO  Configuração carregada
        config_file=config.toml
        target_fps=30
        cameras=2
        vhost=client-a

INFO  Redis Store configurado
        vhost=client-a
        prefix=frames
        ttl_seconds=300
```

## Implementação Técnica

### KeyGenerator

O `KeyGenerator` agora aceita `vhost` ao invés de `instance_id` e usa Unix timestamp:

```go
keyGen := storage.NewKeyGenerator(storage.KeyGeneratorConfig{
    Strategy: storage.StrategySequence,
    Prefix:   "frames",
    Vhost:    "supermercado_vhost",
})

key := keyGen.GenerateKey("cam4", time.Now())
// Resultado: supermercado_vhost:frames:cam4:1731024000123456789:00001
```

### Extração do Vhost

```go
cfg, _ := config.LoadConfig("config.toml")
vhost := cfg.ExtractVhostFromAMQP()
// Extrai vhost da cfg.AMQP.AmqpURL
```

### Parsing de Chaves

```go
// Parse uma chave
vhost, camera, timestamp, seq, err := storage.ParseKey(key)

// timestamp é time.Time reconstruído do Unix nanoseconds
fmt.Printf("Frame capturado em: %s\n", timestamp.Format(time.RFC3339))
```

### Range Queries por Tempo

```go
func GetFramesLastHour(store *storage.RedisStore, cameraID string) ([]string, error) {
    ctx := context.Background()
    
    // Define range de tempo
    endTime := time.Now()
    startTime := endTime.Add(-1 * time.Hour)
    
    // Busca chaves
    pattern := store.keyGenerator.QueryPattern(cameraID, "")
    iter := store.client.Scan(ctx, 0, pattern, 0).Iterator()
    
    var keys []string
    for iter.Next(ctx) {
        key := iter.Val()
        
        // Parse timestamp
        _, _, timestamp, _, err := storage.ParseKey(key)
        if err != nil {
            continue
        }
        
        // Filtrar por range
        if timestamp.After(startTime) && timestamp.Before(endTime) {
            keys = append(keys, key)
        }
    }
    
    return keys, iter.Err()
}
```

### Estratégias de Geração de Chaves

| Estratégia | Formato | Uso Recomendado |
|------------|---------|-----------------|
| `StrategyBasic` | `vhost:prefix:cam:timestamp` | Baixo volume |
| `StrategySequence` | `vhost:prefix:cam:timestamp:seq` | **Recomendado** - Médio/alto volume |
| `StrategyUUID` | `vhost:prefix:cam:timestamp:uuid` | Sistemas distribuídos |

## Migração de Instance ID

Se você estava usando `instance_id` anteriormente:

### Antes
```toml
instance_id = "production-1"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq:5672/myvhost"
```

### Depois
```toml
# instance_id REMOVIDO

[amqp]
# O vhost na URL é usado automaticamente
amqp_url = "amqp://user:pass@rabbitmq:5672/production-1"
```

### Impacto
- **Chaves antigas**: `frames:production-1:cam1:2025-11-08T00:11:01.917Z`
- **Chaves novas**: `production-1:frames:cam1:1731024000917000000:00001`
- ⚠️ **Breaking Change**: Formato completamente diferente
- 🔄 **Migração**: Limpar Redis ou aguardar expiração por TTL

## Monitoramento

### Métricas por Cliente

Use o vhost como label para métricas:

```go
// Exemplo de métrica Prometheus
framesSavedTotal.WithLabelValues(vhost, cameraID).Inc()
```

### Dashboard Grafana

Filtre dados por vhost:

```promql
# Frames salvos por vhost
sum by (vhost) (frames_saved_total)

# FPS por câmera e vhost
rate(frames_saved_total{vhost="client-a"}[5m])
```

## Troubleshooting

### Verificar Vhost Sendo Usado

```bash
# Verifique os logs da aplicação
docker logs edge-video | grep vhost

# Saída esperada:
# INFO Configuração carregada vhost=client-a
# INFO Redis Store configurado vhost=client-a
```

### Chaves Redis não Aparecem

1. **Verifique o vhost na URL AMQP**
   ```bash
   # No config.toml
   amqp_url = "amqp://user:pass@host:5672/SEU_VHOST"
   ```

2. **Verifique o padrão de busca**
   ```bash
   # Use o vhost correto
   redis-cli KEYS "frames:SEU_VHOST:*"
   ```

3. **Verifique se Redis está habilitado**
   ```toml
   [redis]
   enabled = true
   ```

### Colisões Entre Clientes

Se dois clientes usarem o **mesmo vhost**, haverá colisão de chaves. Solução:

```bash
# Cada cliente DEVE ter seu próprio vhost
Cliente A: amqp://host/client-a
Cliente B: amqp://host/client-b
Cliente C: amqp://host/client-c
```

## Referências

- [RabbitMQ Virtual Hosts](https://www.rabbitmq.com/vhosts.html)
- [Redis Key Design](https://redis.io/docs/manual/keyspace/)
- [Multi-tenancy Best Practices](https://www.rabbitmq.com/vhosts.html#virtual-hosts)
