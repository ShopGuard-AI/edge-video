# Message Queue (RabbitMQ)

O **Edge Video** utiliza RabbitMQ como sistema de mensageria para distribuição de frames e metadados de vídeo entre componentes, permitindo arquitetura desacoplada e escalável.

## Visão Geral

O Message Queue oferece:

- ✅ **Publish/Subscribe**: Múltiplos consumers para cada frame
- ✅ **Multi-Tenant**: Isolamento completo por vhost
- ✅ **Durabilidade**: Mensagens persistentes em disco
- ✅ **Routing Flexível**: Exchanges e routing keys configuráveis
- ✅ **Alta Disponibilidade**: Suporte a clustering e replicação

## Arquitetura

```mermaid
graph LR
    A[Camera Collector] -->|Publish| B[RabbitMQ Exchange]
    B -->|Route| C[Queue 1: AI Processing]
    B -->|Route| D[Queue 2: Storage]
    B -->|Route| E[Queue 3: Analytics]
    C -->|Consume| F[AI Consumer]
    D -->|Consume| G[Storage Consumer]
    E -->|Consume| H[Analytics Consumer]
```

**Componentes:**

| Componente | Descrição | Responsabilidade |
|------------|-----------|------------------|
| **Producer** | Camera Collector | Publica frames/metadata |
| **Exchange** | Ponto de roteamento | Distribui mensagens para queues |
| **Queue** | Buffer de mensagens | Armazena mensagens até consumo |
| **Consumer** | Aplicação processadora | Consome e processa mensagens |
| **Vhost** | Namespace isolado | Multi-tenancy |

## Configuração Básica

=== "config.toml"

    ```toml
    [amqp]
    amqp_url = "amqp://user:password@rabbitmq:5672/meu-cliente"
    exchange = "cameras"
    routing_key = "camera.frame"
    
    [metadata]
    enabled = true
    exchange = "camera.metadata"
    routing_key = "camera.metadata.event"
    ```

=== "Docker Compose"

    ```yaml
    services:
      rabbitmq:
        image: rabbitmq:3.13-management
        ports:
          - "5672:5672"    # AMQP
          - "15672:15672"  # Management UI
        environment:
          - RABBITMQ_DEFAULT_USER=admin
          - RABBITMQ_DEFAULT_PASS=admin
        volumes:
          - rabbitmq-data:/var/lib/rabbitmq
      
      camera-collector:
        image: t3labs/edge-video:latest
        depends_on:
          - rabbitmq
        environment:
          - AMQP_URL=amqp://admin:admin@rabbitmq:5672/meu-cliente
    ```

=== "RabbitMQ Setup"

    ```bash
    # Criar vhost para cliente
    rabbitmqctl add_vhost meu-cliente
    
    # Criar usuário
    rabbitmqctl add_user meu-cliente senha_segura
    
    # Dar permissões ao usuário no vhost
    rabbitmqctl set_permissions -p meu-cliente meu-cliente ".*" ".*" ".*"
    
    # Listar vhosts
    rabbitmqctl list_vhosts
    ```

## Tipos de Mensagens

### 1. Frame Completo

Publicado no exchange principal com frame de vídeo completo.

**Exchange:** `cameras`  
**Routing Key:** `camera.frame`  
**Content-Type:** `application/octet-stream`  
**Tamanho:** 200-500 KB (dependendo da resolução)

**Headers:**
```json
{
  "camera_id": "cam4",
  "timestamp": "2024-11-08T14:30:00.123456789Z",
  "vhost": "meu-cliente",
  "content_encoding": "gzip"
}
```

### 2. Metadata Event

Publicado no exchange de metadados com informações sobre o frame.

**Exchange:** `camera.metadata`  
**Routing Key:** `camera.metadata.event`  
**Content-Type:** `application/json`  
**Tamanho:** ~200 bytes

**Payload:**
```json
{
  "camera_id": "cam4",
  "timestamp": "2024-11-08T14:30:00.123456789Z",
  "timestamp_nano": 1731073800123456789,
  "sequence": "00001",
  "redis_key": "meu-cliente:frames:cam4:1731073800123456789:00001",
  "vhost": "meu-cliente",
  "frame_size_bytes": 245678,
  "ttl_seconds": 300
}
```

!!! tip "Escolha do Tipo"
    - Use **Frame Completo** quando o consumer precisa do frame imediatamente
    - Use **Metadata Event** para notificações leves e busque do Redis quando necessário

## Multi-Tenancy com Vhosts

### Conceito

Cada cliente opera em um **vhost isolado** do RabbitMQ, garantindo:

- ✅ Zero conflitos entre clientes
- ✅ Permissões granulares por cliente
- ✅ Métricas e monitoramento separados
- ✅ Configurações independentes

### Configuração por Cliente

```toml
# Cliente A - Supermercado
[amqp]
amqp_url = "amqp://cliente-a:pass@rabbitmq:5672/supermercado"

# Cliente B - Shopping
[amqp]
amqp_url = "amqp://cliente-b:pass@rabbitmq:5672/shopping"

# Cliente C - Condomínio
[amqp]
amqp_url = "amqp://cliente-c:pass@rabbitmq:5672/condominio"
```

**Resultado:**
- Cada cliente tem exchanges, queues e mensagens completamente isoladas
- `supermercado/cameras` ≠ `shopping/cameras` ≠ `condominio/cameras`

### Comandos Úteis

```bash
# Listar exchanges por vhost
rabbitmqctl list_exchanges -p supermercado

# Listar queues por vhost
rabbitmqctl list_queues -p shopping name messages consumers

# Estatísticas de mensagens
rabbitmqctl list_queues -p condominio name messages_ready messages_unacknowledged

# Purgar queue específica
rabbitmqctl purge_queue -p supermercado nome_da_queue
```

## Best Practices

!!! success "Recomendações"
    
    1. **Vhosts**: Sempre use vhosts para isolamento multi-tenant
    2. **Durabilidade**: Configure exchanges e queues como duráveis em produção
    3. **Ack Manual**: Use `auto_ack=False` e faça ack apenas após processar
    4. **Prefetch**: Configure `prefetch_count` baseado no tipo de processamento
    5. **Dead Letter Queue**: Configure DLQ para mensagens com erro
    6. **Connection Pooling**: Reutilize conexões para melhor performance
    7. **Monitoring**: Use Management UI para observabilidade
    8. **Limits**: Configure memory e disk watermarks apropriados
    9. **Retry Logic**: Implemente retry com backoff exponencial
    10. **Graceful Shutdown**: Feche conexões corretamente ao parar consumers

## Monitoramento

### Management UI

Acesse `http://localhost:15672` (user: `admin`, pass: `admin`)

**Dashboards disponíveis:**
- Overview: Visão geral do cluster
- Connections: Conexões ativas
- Channels: Canais por conexão
- Exchanges: Exchanges declaradas
- Queues: Queues e suas métricas
- Admin: Gerenciamento de usuários e vhosts

### Métricas Importantes

```bash
# Total de mensagens por queue
rabbitmqctl list_queues -p meu-cliente name messages

# Taxa de publicação/consumo
rabbitmqctl list_queues -p meu-cliente name messages_ready message_stats

# Consumers ativos
rabbitmqctl list_consumers -p meu-cliente

# Conexões ativas
rabbitmqctl list_connections

# Uso de memória
rabbitmqctl status | grep memory
```

### Alertas Recomendados

| Métrica | Threshold | Ação |
|---------|-----------|------|
| `messages_ready` | > 10000 | Escalar consumers |
| `messages_unacknowledged` | > 1000 | Verificar consumers travados |
| `consumer_utilisation` | < 50% | Otimizar processamento |
| `memory_used` | > 80% | Aumentar RAM ou configurar limites |
| `disk_free` | < 20% | Liberar espaço ou aumentar disco |

## Integração com Edge Video

```mermaid
sequenceDiagram
    participant CC as Camera Collector
    participant MQ as RabbitMQ
    participant R as Redis
    participant C1 as Consumer 1
    participant C2 as Consumer 2
    
    CC->>R: Store frame
    CC->>MQ: Publish metadata
    MQ->>C1: Deliver metadata
    MQ->>C2: Deliver metadata
    C1->>R: Fetch frame
    C2->>R: Fetch frame
    C1->>C1: Process
    C2->>C2: Process
```

Para mais detalhes sobre integração:

<div class="grid cards" markdown>

-   :material-file-document:{ .lg } __Metadata Publisher__
    
    Eventos de metadados via RabbitMQ
    
    [:octicons-arrow-right-24: Metadata Guide](metadata.md)

-   :material-database:{ .lg } __Redis Storage__
    
    Cache de frames com TTL
    
    [:octicons-arrow-right-24: Redis Guide](redis-storage.md)

-   :material-cube-outline:{ .lg } __Multi-Tenancy__
    
    Isolamento por vhost
    
    [:octicons-arrow-right-24: Vhost Guide](../vhost-based-identification.md)

</div>
