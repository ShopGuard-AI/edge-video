# 🔧 FIXES TODO - Goroutine Leak & Redis Issues

## 🎯 OBJETIVO

Resolver o problema de **goroutine leak** e **câmeras mortas silenciosamente** causado por Redis lento.

---

## 📋 PROBLEMAS IDENTIFICADOS

### **PROBLEMA #1: Goroutine Leak no publishLoop** 🔴 CRÍTICO

**Localização**: `camera_stream.go:439-475`

**Root Cause**:
- `publishLoop` cria goroutines assíncronas **sem limite**
- Se Redis está lento (8-15s), goroutines **acumulam** indefinidamente
- `publishWg` rastreia mas **NÃO limita** criação
- Resultado: 100+ goroutines travadas esperando Redis I/O

**Evidência (pprof)**:
```
11 goroutines @ publisher.go:341 → redis_client.go:98
Bloqueadas em: internal/poll.runtime_pollWait
Esperando I/O de rede do Redis
```

---

### **PROBLEMA #2: Redis Store Sem Timeout Configurável** 🟡 ALTO

**Localização**: `redis_client.go:96-98`

**Problemas**:
- Timeout **hardcoded** em 5 segundos
- Com 3 retries = **até 15 segundos bloqueado**
- Não respeita contexto de cancelamento da câmera
- Redis remoto lento = goroutines acumulam indefinidamente

---

### **PROBLEMA #3: Circuit Breaker Só Detecta Falhas de FFmpeg** 🟡 ALTO

**Localização**: `camera_stream.go:228`, `camera_stream.go:283`

**Comportamento Atual**:
- Circuit Breaker **SÓ rastreia** erros de `startFFmpeg()` e `readFrames()`
- **NÃO detecta** goroutines travadas em `Publish()`
- **NÃO detecta** se publicação está falhando sistematicamente

**Resultado**: Câmera "morre silenciosamente" sem retry!

---

### **PROBLEMA #4: Sem Limite de Goroutines Concorrentes** 🔴 CRÍTICO

**Comportamento Atual**:
```go
publishWg.Add(1)  // ← SEM LIMITE!
go func(...) {
    err := c.publisher.Publish(...)  // Pode travar 15s
}(...)
```

**Resultado**: Com Redis lento, 100+ goroutines criadas em 10 segundos!

---

## 🏗️ SOLUÇÃO ARQUITETURAL

### **LAYER 1: Publish Worker Pool** ⭐ CORE (PR #1)

**Implementação**:

```go
// camera_stream.go
type CameraStream struct {
    // ... campos existentes ...

    publishSemaphore chan struct{}  // Limita workers concorrentes
    publishQueue     chan publishJob // Fila de jobs
}

type publishJob struct {
    cameraID  string
    frameData []byte
    frameNum  uint64
    timestamp time.Time
}

// Configurável
const maxConcurrentPublishers = 3  // Máximo 3 goroutines simultâneas
```

**Mudanças no publishLoop**:

```go
func (c *CameraStream) publishLoop() {
    // Cria semáforo (3 slots)
    c.publishSemaphore = make(chan struct{}, 3)

    for {
        // ... pega frame ...

        // Tenta adquirir slot (NON-BLOCKING)
        select {
        case c.publishSemaphore <- struct{}{}:
            // Slot adquirido, pode criar goroutine
            publishWg.Add(1)
            go func(...) {
                defer func() {
                    <-c.publishSemaphore  // Libera slot
                    publishWg.Done()
                }()

                err := c.publisher.Publish(...)
            }(...)

        default:
            // Sem slots disponíveis, DESCARTA frame
            log.Printf("[%s] Publish queue full, dropping frame", c.ID)
        }
    }
}
```

**Benefícios**:
- ✅ Máximo **3 goroutines travadas** por câmera (vs 100+)
- ✅ Backpressure automático
- ✅ Sistema continua funcionando mesmo com Redis lento

---

### **LAYER 2: Context-Aware Redis** ⭐ CORE (PR #1)

**Mudanças em `redis_client.go`**:

```go
// RedisConfig (adicionar)
type RedisConfig struct {
    // ... campos existentes ...
    StoreTimeout time.Duration `yaml:"store_timeout"`  // NOVO
}

// Store ANTES
func (r *RedisClient) Store(cameraID string, frameData []byte, timestamp time.Time) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)  // ← PROBLEMA
    // ...
}

// Store DEPOIS
func (r *RedisClient) StoreWithContext(ctx context.Context, cameraID string, frameData []byte, timestamp time.Time) (string, error) {
    // Usa contexto do CALLER + timeout configurável
    storeCtx, cancel := context.WithTimeout(ctx, r.config.StoreTimeout)
    defer cancel()

    err := r.client.Set(storeCtx, key, frameData, r.config.TTL).Err()

    // Se ctx.Done() (câmera parou) → retorna IMEDIATAMENTE
    // ...
}
```

**Mudanças em `publisher.go`**:

```go
// Publish ANTES
func (p *Publisher) Publish(cameraID string, frameData []byte, timestamp time.Time) error {
    redisKey, storeErr = p.redisClient.Store(cameraID, frameDataCopy, timestamp)
}

// Publish DEPOIS
func (p *Publisher) PublishWithContext(ctx context.Context, cameraID string, frameData []byte, timestamp time.Time) error {
    redisKey, storeErr = p.redisClient.StoreWithContext(ctx, cameraID, frameDataCopy, timestamp)
}
```

**Mudanças em `camera_stream.go`**:

```go
// publishLoop - passa contexto da câmera
go func(...) {
    err := c.publisher.PublishWithContext(c.ctx, cameraID, frameData, start)
    //                                    ↑ contexto da câmera
}(...)
```

**Benefícios**:
- ✅ Se câmera para → Redis cancela **imediatamente**
- ✅ Timeout configurável via YAML (2s vs 5s hardcoded)
- ✅ Goroutines não ficam "órfãs" após shutdown

---

### **LAYER 3: Publish Health Monitor** ⭐ INNOVATION (PR #2)

**Novo arquivo**: `internal/health/publish_health.go`

```go
package health

type PublishHealthMonitor struct {
    mu                sync.Mutex
    recentErrors      []time.Time     // Últimos 10 erros
    recentSuccesses   []time.Time     // Últimos 10 sucessos
    consecutiveFails  int
    lastSuccess       time.Time
    windowSize        int             // Janela de análise (padrão: 10)
    degradedThreshold float64         // Threshold (padrão: 0.8 = 80% erros)
}

func (p *PublishHealthMonitor) RecordError() {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.recentErrors = append(p.recentErrors, time.Now())
    if len(p.recentErrors) > p.windowSize {
        p.recentErrors = p.recentErrors[1:]
    }
    p.consecutiveFails++
}

func (p *PublishHealthMonitor) RecordSuccess() {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.recentSuccesses = append(p.recentSuccesses, time.Now())
    if len(p.recentSuccesses) > p.windowSize {
        p.recentSuccesses = p.recentSuccesses[1:]
    }
    p.consecutiveFails = 0
    p.lastSuccess = time.Now()
}

func (p *PublishHealthMonitor) IsDegraded() bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Calcula taxa de erro nos últimos 10 publishes
    total := len(p.recentErrors) + len(p.recentSuccesses)
    if total < p.windowSize {
        return false  // Dados insuficientes
    }

    errorRate := float64(len(p.recentErrors)) / float64(total)

    // Degraded se:
    // - 80%+ de erros nos últimos 10 publishes
    // OU
    // - Sem sucesso há 30 segundos
    return errorRate >= p.degradedThreshold ||
           time.Since(p.lastSuccess) > 30*time.Second
}
```

**Integração em `camera_stream.go`**:

```go
type CameraStream struct {
    // ... campos existentes ...
    publishHealth *health.PublishHealthMonitor
}

func (c *CameraStream) publishLoop() {
    for {
        // ... publica ...

        if err != nil {
            c.publishHealth.RecordError()

            // NOVO: Detecta degradação e aciona Circuit Breaker
            if c.publishHealth.IsDegraded() {
                log.Printf("[%s] ⚠️  Publish health degraded, recording CB failure", c.ID)
                c.circuitBreaker.RecordFailure()
            }
        } else {
            c.publishHealth.RecordSuccess()
        }
    }
}
```

**Benefícios**:
- ✅ Detecta "câmera morta" mesmo com FFmpeg vivo
- ✅ Aciona Circuit Breaker automaticamente
- ✅ Retry automático via backoff exponencial

---

### **LAYER 4: Circuit Breaker Híbrido** ⭐ ENHANCEMENT (PR #2)

**Mudanças em `camera_stream.go`**:

```go
// startFFmpeg - JÁ registra falhas de FFmpeg
if err != nil {
    c.circuitBreaker.Execute(func() error { return err })
}

// publishLoop - AGORA TAMBÉM registra falhas de Publish
if c.publishHealth.IsDegraded() {
    c.circuitBreaker.RecordFailure()  // ← NOVO
}
```

**Benefícios**:
- ✅ Circuit Breaker abre se **FFmpeg falha** OU **Publish degrada**
- ✅ Retry unificado para ambos os problemas
- ✅ Visibilidade completa do estado da câmera

---

### **LAYER 5: Graceful Degradation** ⭐ RESILIENCE (PR #3)

**A) Dynamic FPS Throttling**:

```go
func (c *CameraStream) publishLoop() {
    interval := time.Second / time.Duration(c.FPS)

    for {
        // Ajusta FPS se publish degradado
        if c.publishHealth.IsDegraded() {
            interval = time.Second / 5  // Throttle para 5 FPS
            log.Printf("[%s] Publish degraded, throttling to 5 FPS", c.ID)
        } else {
            interval = time.Second / time.Duration(c.FPS)  // FPS normal
        }

        // ... continua ...
    }
}
```

**B) Emergency Mode**:

```go
// Se 20 failures consecutivos → força restart
if c.publishHealth.consecutiveFails > 20 {
    log.Printf("[%s] 🚨 EMERGENCY: 20 consecutive publish failures, forcing restart", c.ID)
    c.circuitBreaker.ForceOpen()  // Força FFmpeg restart
}
```

---

## ⚙️ CONFIGURAÇÃO YAML

```yaml
# config.yaml - NOVAS CONFIGURAÇÕES

# Camera Stream (novo)
camera_stream:
  max_publish_workers: 3          # Máximo de goroutines de publish simultâneas
  publish_queue_size: 10          # Tamanho da fila de publish (não usado com semáforo)
  publish_health_window: 10       # Janela de análise (últimos N publishes)
  publish_health_threshold: 0.8   # 80% de erros = degraded
  emergency_failures: 20          # Failures consecutivos para forçar restart
  degraded_fps: 5                 # FPS quando degradado (throttling)

# Redis (atualizado)
redis:
  enabled: true
  address: "34.30.59.236:6379"
  password: "MinhaSenhaMuitoForte@2024"
  db: 0
  vhost: "supercarlao_rj_mercado"
  prefix: "frames"
  ttl: 120s
  store_timeout: 2s               # NOVO: Timeout por tentativa (era 5s hardcoded)
  max_retries: 2                  # Reduzido de 3 para 2
  retry_delay: 50ms               # Reduzido de 100ms para 50ms
```

---

## 📊 COMPARAÇÃO: ANTES vs DEPOIS

| Aspecto | ANTES (Atual) | DEPOIS (Solução) |
|---------|---------------|------------------|
| **Goroutines max** | Ilimitadas (100+) | 3 por câmera |
| **Redis timeout** | 5s × 3 = 15s | 2s × 2 = 4s |
| **Detecção falha publish** | ❌ Não detecta | ✅ PublishHealthMonitor |
| **Circuit Breaker scope** | Apenas FFmpeg | FFmpeg + Publish |
| **Goroutines órfãs** | ✅ Sim (leak) | ❌ Não (context cancela) |
| **Shutdown time** | 40s+ (timeout) | <5s (graceful) |
| **Recovery automático** | ❌ Não (só FFmpeg) | ✅ Sim (FFmpeg + Publish) |
| **Degradação graceful** | ❌ Não | ✅ Throttling automático |

---

## 🚀 PLANO DE IMPLEMENTAÇÃO

### **PR #1: Core Fixes** (CRÍTICO - IMPLEMENTAR PRIMEIRO)

**Prioridade**: 🔴 CRÍTICA

**Arquivos modificados**:
- `internal/stream/camera_stream.go` (+80 linhas)
  - Worker pool com semáforo
  - Context-aware publish

- `internal/storage/redis_client.go` (+30 linhas)
  - `StoreWithContext()` com contexto do caller
  - Timeout configurável

- `internal/messaging/publisher.go` (+20 linhas)
  - `PublishWithContext()` que passa contexto para Redis

- `config.go` (+10 linhas)
  - `CameraStreamConfig` struct
  - `redis.store_timeout` field

**Testes necessários**:
1. Redis lento artificial (10s delay)
2. Shutdown com Redis travado
3. Múltiplas câmeras simultâneas

**Resultado esperado**:
- ✅ Máximo 3 goroutines travadas por câmera
- ✅ Shutdown em <5s
- ✅ Sem goroutines órfãs

---

### **PR #2: Health Monitoring** (ALTO)

**Prioridade**: 🟡 ALTA

**Arquivos novos**:
- `internal/health/publish_health.go` (+150 linhas)
  - PublishHealthMonitor struct
  - IsDegraded() logic

**Arquivos modificados**:
- `internal/stream/camera_stream.go` (+40 linhas)
  - Integração com PublishHealthMonitor
  - Circuit Breaker híbrido

- `internal/metrics/metrics.go` (+20 linhas)
  - Métricas de health status

**Testes necessários**:
1. Simular 80% de falhas
2. Verificar Circuit Breaker abre
3. Verificar recovery automático

**Resultado esperado**:
- ✅ Detecta degradação em <10s
- ✅ Circuit Breaker abre automaticamente
- ✅ Recovery sem intervenção manual

---

### **PR #3: Graceful Degradation** (MÉDIO)

**Prioridade**: 🟢 MÉDIA

**Arquivos modificados**:
- `internal/stream/camera_stream.go` (+50 linhas)
  - Dynamic FPS throttling
  - Emergency mode

- `config.yaml` (+10 linhas)
  - `degraded_fps` config
  - `emergency_failures` config

**Testes necessários**:
1. Verificar throttling quando degraded
2. Verificar emergency mode após 20 failures
3. Verificar FPS volta ao normal após recovery

**Resultado esperado**:
- ✅ FPS reduz automaticamente quando degraded
- ✅ Sistema continua funcionando
- ✅ Recovery gradual quando Redis melhora

---

## 🧪 TESTES DE VALIDAÇÃO

### **Teste 1: Redis Lento (10s latência)**

```bash
# Linux (tc)
tc qdisc add dev eth0 root netem delay 10000ms

# Windows (PowerShell - simular via firewall rules)
# Adicionar regra para adicionar delay
```

**Resultado esperado**:
- ✅ Máximo 3 goroutines travadas por câmera
- ✅ PublishHealthMonitor detecta degradação em 10s
- ✅ Circuit Breaker abre após 5 failures
- ✅ FPS throttle para 5 FPS
- ✅ Sistema continua funcionando (cam2 OK)

---

### **Teste 2: Redis Completamente Down**

```bash
# Para Redis
systemctl stop redis  # Linux
net stop Redis        # Windows
```

**Resultado esperado**:
- ✅ Publish falha imediatamente (timeout 2s)
- ✅ Health Monitor detecta em <10s
- ✅ Circuit Breaker abre
- ✅ FFmpeg para e entra em retry backoff (5s → 10s → 20s...)

---

### **Teste 3: Recovery Automático**

```bash
# Redis volta
systemctl start redis  # Linux
net start Redis        # Windows
```

**Resultado esperado**:
- ✅ Próximo publish tenta (circuit breaker em HALF_OPEN)
- ✅ 3 sucessos consecutivos → Circuit Breaker fecha
- ✅ FPS volta ao normal (5 → 15 FPS)
- ✅ Câmera recupera SEM restart manual

---

### **Teste 4: Shutdown Graceful**

```bash
# Ctrl+C no producer com Redis travado
```

**Resultado esperado**:
- ✅ Contexto cancela todas as goroutines
- ✅ Redis.Store() retorna imediatamente
- ✅ publishWg.Wait() completa em <5s
- ✅ Nenhuma goroutine órfã

---

## 📝 CHECKLIST DE IMPLEMENTAÇÃO

### **PR #1: Core Fixes**
- [ ] Implementar `publishSemaphore` em `CameraStream`
- [ ] Modificar `publishLoop` para usar semáforo
- [ ] Adicionar `StoreWithContext()` em `RedisClient`
- [ ] Adicionar `PublishWithContext()` em `Publisher`
- [ ] Adicionar `store_timeout` em `RedisConfig`
- [ ] Adicionar `CameraStreamConfig` em `config.go`
- [ ] Atualizar `config.yaml` com novos fields
- [ ] Testar com Redis lento
- [ ] Testar shutdown graceful
- [ ] Rebuild binary
- [ ] Commit & Push

### **PR #2: Health Monitoring**
- [ ] Criar `internal/health/publish_health.go`
- [ ] Implementar `PublishHealthMonitor` struct
- [ ] Implementar `IsDegraded()` logic
- [ ] Integrar em `CameraStream`
- [ ] Conectar com Circuit Breaker
- [ ] Adicionar métricas Prometheus
- [ ] Testar detecção de degradação
- [ ] Testar recovery automático
- [ ] Rebuild binary
- [ ] Commit & Push

### **PR #3: Graceful Degradation**
- [ ] Implementar Dynamic FPS throttling
- [ ] Implementar Emergency mode
- [ ] Adicionar configs em YAML
- [ ] Testar throttling
- [ ] Testar emergency mode
- [ ] Testar recovery gradual
- [ ] Rebuild binary
- [ ] Commit & Push

---

## 🎯 MÉTRICAS DE SUCESSO

Após implementação completa, devemos observar:

| Métrica | Antes | Depois | Status |
|---------|-------|--------|--------|
| Goroutines travadas (Redis lento) | 100+ | ≤3 | ⏳ |
| Tempo de shutdown (Redis travado) | 40s+ | <5s | ⏳ |
| Detecção de câmera morta | Manual | <10s auto | ⏳ |
| Recovery após Redis volta | Manual restart | Automático | ⏳ |
| FPS durante degradação | 0 (travado) | 5 (throttled) | ⏳ |

---

## 📚 REFERÊNCIAS

- **Goroutine leak analysis**: `pprof` output showing 11 goroutines stuck
- **Root cause**: `camera_stream.go:439-475` unlimited goroutine creation
- **Redis blocking**: `redis_client.go:96-98` hardcoded 5s timeout
- **Circuit Breaker gap**: Only monitors FFmpeg, not Publish

---

**Data de criação**: 2024-12-09
**Prioridade**: 🔴 CRÍTICA
**Estimativa**: PR#1 (4h), PR#2 (3h), PR#3 (2h) = **9 horas total**

