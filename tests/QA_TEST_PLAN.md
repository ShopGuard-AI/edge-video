# QA Test Plan - Edge Video V2
## Test Strategy & Coverage Report

**Version:** 2.0
**Date:** 2025-12-09
**Status:** ✅ Fase 1 Completa | 🚀 Fase 2 Em Progresso
**Coverage Target:** 85%+

---

## 📊 STATUS GERAL

| Fase | Status | Testes | Cobertura | Bugs Encontrados | Bugs Corrigidos |
|------|--------|--------|-----------|------------------|-----------------|
| **Fase 1 - Unit Tests** | ✅ **COMPLETO** | 43/43 (100%) | 57% | 2 críticos | ✅ 2/2 (100%) |
| **Fase 2 - Integration Tests** | 🚀 **EM PROGRESSO** | 0/8 | 0% | 0 | 0 |
| **Fase 3 - Stress Tests** | ⏳ Pendente | 0/4 | 0% | 0 | 0 |
| **Fase 4 - E2E Tests** | ⏳ Pendente | 0/2 | 0% | 0 | 0 |
| **Fase 5 - CI/CD** | ⏳ Pendente | - | - | 0 | 0 |

**Última Atualização**: 2025-12-09 22:57 UTC

---

## Test Levels

```
┌─────────────────────────────────────────────────────────┐
│                    E2E Tests (5%)                        │
│  Test complete workflows from camera to consumer        │
├─────────────────────────────────────────────────────────┤
│              Integration Tests (25%)                     │
│   Test interaction between components                   │
├─────────────────────────────────────────────────────────┤
│               Unit Tests (60%)                           │
│    Test individual functions and methods                │
├─────────────────────────────────────────────────────────┤
│           Stress/Load Tests (10%)                        │
│  Test system under heavy load and edge cases            │
└─────────────────────────────────────────────────────────┘
```

---

## 1. UNIT TESTS (60% coverage)

### 1.1 Health Monitoring (`internal/health/publish_health_test.go`)

#### Test: TestNewPublishHealthMonitor
**Objetivo:** Validar criação do monitor com configurações corretas
**Como testar:**
```go
func TestNewPublishHealthMonitor(t *testing.T) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    assert.NotNil(t, monitor)
    assert.Equal(t, 10, monitor.windowSize)
    assert.Equal(t, 0.8, monitor.degradedThreshold)
    assert.Equal(t, 0, len(monitor.recentErrors))
}
```
**Resultado esperado:** Monitor criado com valores corretos, slices vazios

---

#### Test: TestRecordSuccessAndError
**Objetivo:** Verificar registro de sucessos e erros
**Como testar:**
```go
func TestRecordSuccessAndError(t *testing.T) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    // Record 3 successes
    monitor.RecordSuccess()
    monitor.RecordSuccess()
    monitor.RecordSuccess()

    stats := monitor.GetStats()
    assert.Equal(t, 3, stats.RecentSuccesses)
    assert.Equal(t, 0, stats.RecentErrors)
    assert.Equal(t, 0, stats.ConsecutiveFails)

    // Record 2 errors
    monitor.RecordError()
    monitor.RecordError()

    stats = monitor.GetStats()
    assert.Equal(t, 3, stats.RecentSuccesses)
    assert.Equal(t, 2, stats.RecentErrors)
    assert.Equal(t, 2, stats.ConsecutiveFails)
}
```
**Resultado esperado:** Sucessos/erros registrados corretamente, consecutiveFails atualizado

---

#### Test: TestIsDegradedByErrorRate
**Objetivo:** Verificar detecção de degradação por taxa de erro
**Como testar:**
```go
func TestIsDegradedByErrorRate(t *testing.T) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    // Fill window: 8 errors, 2 successes = 80% error rate
    for i := 0; i < 8; i++ {
        monitor.RecordError()
    }
    for i := 0; i < 2; i++ {
        monitor.RecordSuccess()
    }

    assert.True(t, monitor.IsDegraded())
    stats := monitor.GetStats()
    assert.Equal(t, 0.8, stats.ErrorRate)
    assert.True(t, stats.IsDegraded)
}
```
**Resultado esperado:** IsDegraded() retorna true com 80%+ de erros

---

#### Test: TestIsEmergency
**Objetivo:** Verificar detecção de estado de emergência (>20 falhas consecutivas)
**Como testar:**
```go
func TestIsEmergency(t *testing.T) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    // Record 21 consecutive failures
    for i := 0; i < 21; i++ {
        monitor.RecordError()
    }

    assert.True(t, monitor.IsEmergency())
    stats := monitor.GetStats()
    assert.Equal(t, 21, stats.ConsecutiveFails)
    assert.True(t, stats.IsEmergency)
}
```
**Resultado esperado:** IsEmergency() retorna true após >20 falhas

---

#### Test: TestNoDeadlock
**Objetivo:** Verificar que GetStats() NÃO causa deadlock (fix PR#2)
**Como testar:**
```go
func TestNoDeadlock(t *testing.T) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    done := make(chan bool)

    // Goroutine 1: Record success continuously
    go func() {
        for i := 0; i < 1000; i++ {
            monitor.RecordSuccess()
            time.Sleep(1 * time.Millisecond)
        }
    }()

    // Goroutine 2: Get stats continuously
    go func() {
        for i := 0; i < 1000; i++ {
            _ = monitor.GetStats()
            time.Sleep(1 * time.Millisecond)
        }
        done <- true
    }()

    // Should complete without deadlock
    select {
    case <-done:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("Deadlock detected!")
    }
}
```
**Resultado esperado:** Teste completa em <5s sem deadlock

---

### 1.2 Circuit Breaker (`internal/resilience/circuit_breaker_test.go`)

#### Test: TestCircuitBreakerStateTransitions
**Objetivo:** Validar transições de estado (CLOSED → OPEN → HALF_OPEN → CLOSED)
**Como testar:**
```go
func TestCircuitBreakerStateTransitions(t *testing.T) {
    cb := NewCircuitBreaker("test", 3, 5*time.Second)

    // Initial state: CLOSED
    assert.Equal(t, CLOSED, cb.GetState())

    // 3 failures → OPEN
    for i := 0; i < 3; i++ {
        cb.Execute(func() error { return errors.New("fail") })
    }
    assert.Equal(t, OPEN, cb.GetState())

    // Wait for timeout → HALF_OPEN
    time.Sleep(6 * time.Second)
    cb.Execute(func() error { return nil })
    assert.Equal(t, HALF_OPEN, cb.GetState())

    // Success → CLOSED
    cb.Execute(func() error { return nil })
    assert.Equal(t, CLOSED, cb.GetState())
}
```
**Resultado esperado:** CB transita corretamente entre estados

---

### 1.3 Redis Client (`internal/storage/redis_client_test.go`)

#### Test: TestStoreWithContextCancellation
**Objetivo:** Verificar que operação é interrompida quando contexto é cancelado
**Como testar:**
```go
func TestStoreWithContextCancellation(t *testing.T) {
    client := NewRedisClient(config)

    ctx, cancel := context.WithCancel(context.Background())

    // Cancel immediately
    cancel()

    _, err := client.StoreWithContext(ctx, "cam1", []byte("test"), time.Now())

    assert.Error(t, err)
    assert.Equal(t, context.Canceled, err)
}
```
**Resultado esperado:** StoreWithContext retorna context.Canceled

---

#### Test: TestStoreWithTimeout
**Objetivo:** Verificar timeout de operação
**Como testar:**
```go
func TestStoreWithTimeout(t *testing.T) {
    client := NewRedisClient(config)

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()

    // Large data to force timeout
    largeData := make([]byte, 10*1024*1024) // 10MB

    _, err := client.StoreWithContext(ctx, "cam1", largeData, time.Now())

    assert.Error(t, err)
}
```
**Resultado esperado:** Timeout ocorre corretamente

---

### 1.4 Publisher (`internal/messaging/publisher_test.go`)

#### Test: TestPublishWithContextCancellation
**Objetivo:** Verificar interrupção de publish quando contexto cancelado
**Como testar:**
```go
func TestPublishWithContextCancellation(t *testing.T) {
    publisher := NewPublisher(config)

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    err := publisher.PublishWithContext(ctx, "cam1", []byte("test"), time.Now())

    assert.Error(t, err)
    assert.Equal(t, context.Canceled, err)
}
```
**Resultado esperado:** PublishWithContext retorna context.Canceled

---

## 2. INTEGRATION TESTS (25% coverage)

### 2.1 Producer → Redis → RabbitMQ (`tests/integration/producer_flow_test.go`)

**Objetivo:** Testar fluxo completo Producer → Redis → RabbitMQ

**Setup:**
- Redis rodando (34.30.59.236:6379)
- RabbitMQ rodando (34.30.59.236:5672)
- Config válido

**Como testar:**
```go
func TestProducerFlow(t *testing.T) {
    // 1. Start producer
    producer := startProducer(t)
    defer producer.Stop()

    // 2. Wait for frames to publish
    time.Sleep(10 * time.Second)

    // 3. Check Redis keys exist
    keys, err := redisClient.Keys("supercarlao_rj_mercado:frames:*").Result()
    assert.NoError(t, err)
    assert.Greater(t, len(keys), 100) // Should have published >100 frames

    // 4. Check RabbitMQ queue has messages
    queueInfo := rabbitClient.QueueInspect("supercarlao_rj_mercado.cam1")
    assert.Greater(t, queueInfo.Messages, 100)

    // 5. Verify frame data in Redis
    frameData, err := redisClient.Get(keys[0]).Bytes()
    assert.NoError(t, err)
    assert.Greater(t, len(frameData), 1000) // Frame should be >1KB
}
```

**Resultado esperado:**
- ✅ Frames armazenados no Redis (>100 frames)
- ✅ Mensagens publicadas no RabbitMQ (>100 msgs)
- ✅ Frame data válido no Redis (JPEG data)

---

### 2.2 Health Monitor Integration (`tests/integration/health_monitor_test.go`)

**Objetivo:** Testar integração do Health Monitor com Circuit Breaker

**Como testar:**
```go
func TestHealthMonitorTriggersCircuitBreaker(t *testing.T) {
    // 1. Create camera with health monitor + circuit breaker
    cam := NewCameraStream(config)

    // 2. Simulate 80% publish errors (trigger degradation)
    for i := 0; i < 10; i++ {
        if i < 8 {
            cam.publishHealth.RecordError()
        } else {
            cam.publishHealth.RecordSuccess()
        }
    }

    // 3. Check health is degraded
    assert.True(t, cam.publishHealth.IsDegraded())

    // 4. Trigger circuit breaker manually
    err := cam.circuitBreaker.Execute(func() error {
        return errors.New("publish degraded")
    })

    // 5. Verify circuit breaker opened
    assert.Equal(t, OPEN, cam.circuitBreaker.GetState())
}
```

**Resultado esperado:**
- ✅ Health monitor detecta degradação (80% erros)
- ✅ Circuit breaker abre após degradação

---

### 2.3 Graceful Shutdown (`tests/integration/shutdown_test.go`)

**Objetivo:** Verificar shutdown gracioso em <5 segundos (PR#1)

**Como testar:**
```go
func TestGracefulShutdown(t *testing.T) {
    // 1. Start producer with 5 cameras
    producer := startProducer(t)

    // 2. Wait for cameras to start publishing
    time.Sleep(5 * time.Second)

    // 3. Send SIGINT
    start := time.Now()
    producer.Stop()
    duration := time.Since(start)

    // 4. Verify shutdown completed in <5s
    assert.Less(t, duration, 5*time.Second)

    // 5. Check goroutines cleaned up
    goroutineCount := runtime.NumGoroutine()
    assert.Less(t, goroutineCount, 10) // Should drop to <10 goroutines
}
```

**Resultado esperado:**
- ✅ Shutdown completa em <5 segundos
- ✅ Todas goroutines encerradas
- ✅ Sem timeout de 45s

---

## 3. STRESS/LOAD TESTS (10% coverage)

### 3.1 High Frame Rate Stress Test (`tests/stress/high_fps_test.go`)

**Objetivo:** Testar sistema com taxa de FPS alta (30 FPS, 10 câmeras)

**Setup:**
- 10 câmeras simultâneas
- 30 FPS cada
- Duração: 5 minutos

**Como testar:**
```go
func TestHighFPSStress(t *testing.T) {
    // 1. Configure 10 cameras @ 30 FPS
    config := loadConfig("high_fps_config.yaml")
    producer := startProducer(t, config)
    defer producer.Stop()

    // 2. Run for 5 minutes
    runtime := 5 * time.Minute
    ticker := time.NewTicker(10 * time.Second)
    done := time.After(runtime)

    for {
        select {
        case <-ticker.C:
            // Check stats
            stats := producer.GetStats()
            t.Logf("Published: %d, Errors: %d, ErrorRate: %.2f%%",
                stats.Published, stats.Errors, stats.ErrorRate*100)

            // Verify error rate <5%
            assert.Less(t, stats.ErrorRate, 0.05)

        case <-done:
            // Final check
            finalStats := producer.GetStats()
            expectedFrames := 10 * 30 * 300 // 10 cams * 30 FPS * 300s
            assert.Greater(t, finalStats.Published, expectedFrames*0.95) // 95%+ success
            return
        }
    }
}
```

**Resultado esperado:**
- ✅ Sistema mantém 30 FPS/câmera por 5 minutos
- ✅ Taxa de erro <5%
- ✅ >95% dos frames publicados com sucesso
- ✅ Sem memory leaks

---

### 3.2 Memory Leak Test (`tests/stress/memory_leak_test.go`)

**Objetivo:** Verificar que não há vazamento de memória durante operação contínua

**Como testar:**
```go
func TestMemoryLeak(t *testing.T) {
    producer := startProducer(t)
    defer producer.Stop()

    // 1. Get baseline memory
    runtime.GC()
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)
    baselineAlloc := m1.Alloc

    // 2. Run for 10 minutes
    time.Sleep(10 * time.Minute)

    // 3. Check memory after run
    runtime.GC()
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)
    finalAlloc := m2.Alloc

    // 4. Verify memory increase <20%
    increase := float64(finalAlloc-baselineAlloc) / float64(baselineAlloc)
    assert.Less(t, increase, 0.20) // <20% increase
}
```

**Resultado esperado:**
- ✅ Aumento de memória <20% após 10 minutos
- ✅ Sem goroutine leak (count estável)

---

### 3.3 Reconnection Stress Test (`tests/stress/reconnection_test.go`)

**Objetivo:** Testar reconexão automática sob stress

**Como testar:**
```go
func TestReconnectionStress(t *testing.T) {
    producer := startProducer(t)
    defer producer.Stop()

    // Simulate 10 connection drops
    for i := 0; i < 10; i++ {
        // Drop RabbitMQ connection
        dropRabbitMQConnection(t)

        // Wait for reconnection
        time.Sleep(10 * time.Second)

        // Verify producer still publishing
        stats := producer.GetStats()
        previousPublished := stats.Published

        time.Sleep(5 * time.Second)

        newStats := producer.GetStats()
        assert.Greater(t, newStats.Published, previousPublished)
    }
}
```

**Resultado esperado:**
- ✅ Producer reconecta automaticamente após cada queda
- ✅ Publicação continua após reconexão
- ✅ Circuit breaker funciona corretamente

---

## 4. E2E TESTS (5% coverage)

### 4.1 Full Pipeline Test (`tests/e2e/full_pipeline_test.go`)

**Objetivo:** Testar pipeline completo: Camera → Producer → Redis → RabbitMQ → Consumer → JPEG salvo

**Setup:**
- Producer rodando com 1 câmera real (RTSP)
- Redis rodando
- RabbitMQ rodando
- Consumer rodando (Python)

**Como testar:**
```python
def test_full_pipeline():
    # 1. Start producer (1 camera)
    producer = start_producer()

    # 2. Start consumer
    consumer = start_consumer()

    # 3. Wait for frames to flow
    time.sleep(30)

    # 4. Check consumer saved frames
    saved_frames = glob.glob("output/cam1_*.jpg")
    assert len(saved_frames) > 100  # Should have >100 frames

    # 5. Verify JPEG integrity
    for frame_path in saved_frames[:10]:
        img = cv2.imread(frame_path)
        assert img is not None
        assert img.shape[0] > 0  # Height > 0
        assert img.shape[1] > 0  # Width > 0

    # 6. Verify latency <500ms
    latencies = measure_latencies(consumer, 100)
    avg_latency = sum(latencies) / len(latencies)
    assert avg_latency < 500  # <500ms avg latency
```

**Resultado esperado:**
- ✅ >100 frames salvos pelo consumer
- ✅ JPEGs válidos e decodificáveis
- ✅ Latência média <500ms
- ✅ Pipeline completo funcionando end-to-end

---

## 5. PERFORMANCE BENCHMARKS

### 5.1 Publish Performance (`tests/benchmarks/publish_bench_test.go`)

**Objetivo:** Medir tempo de publish

**Como testar:**
```go
func BenchmarkPublish(b *testing.B) {
    publisher := NewPublisher(config)
    frameData := make([]byte, 100*1024) // 100KB frame

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        publisher.Publish("cam1", frameData, time.Now())
    }
}
```

**Resultado esperado:**
- Target: <200ms per publish

---

### 5.2 Health Monitor Performance (`tests/benchmarks/health_bench_test.go`)

**Como testar:**
```go
func BenchmarkHealthMonitorGetStats(b *testing.B) {
    monitor := NewPublishHealthMonitor(10, 0.8)

    // Fill with data
    for i := 0; i < 10; i++ {
        monitor.RecordSuccess()
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = monitor.GetStats()
    }
}
```

**Resultado esperado:**
- Target: <1µs per GetStats() call

---

## 6. TEST EXECUTION PLAN

### Step 1: Run Unit Tests
```bash
cd v2
go test ./internal/... -v -cover -coverprofile=coverage.out
```

### Step 2: Run Integration Tests
```bash
go test ./tests/integration/... -v -timeout 30m
```

### Step 3: Run Stress Tests
```bash
go test ./tests/stress/... -v -timeout 60m
```

### Step 4: Run E2E Tests
```bash
python tests/e2e/full_pipeline_test.py
```

### Step 5: Generate Coverage Report
```bash
go tool cover -html=coverage.out -o tests/reports/coverage.html
```

### Step 6: Run Benchmarks
```bash
go test ./tests/benchmarks/... -bench=. -benchmem
```

---

## 7. SUCCESS CRITERIA

| Category | Metric | Target | Critical |
|----------|--------|--------|----------|
| Unit Tests | Coverage | >80% | Yes |
| Unit Tests | Pass Rate | 100% | Yes |
| Integration | Pass Rate | 100% | Yes |
| Integration | Shutdown Time | <5s | Yes |
| Stress | Error Rate | <5% | Yes |
| Stress | Memory Leak | <20% increase | Yes |
| E2E | Latency | <500ms avg | No |
| E2E | Frame Loss | <1% | Yes |
| Benchmarks | Publish Time | <200ms | No |

---

## 8. KNOWN ISSUES & EDGE CASES TO TEST

### 8.1 Deadlock Prevention (PR#2)
- ✅ **Fixed:** GetStats() no longer causes deadlock
- **Test:** TestNoDeadlock (concurrent RecordSuccess + GetStats)

### 8.2 Context Cancellation (PR#1)
- ✅ **Fixed:** PublishWithContext respects context cancellation
- **Test:** TestPublishWithContextCancellation

### 8.3 Goroutine Leak
- ✅ **Fixed:** Goroutines properly cleaned up on shutdown
- **Test:** TestGracefulShutdown (verify goroutine count)

---

## 9. TEST DATA FIXTURES

Create test fixtures in `tests/fixtures/`:

1. **sample_frame.jpg** - Sample JPEG frame (100KB)
2. **large_frame.jpg** - Large JPEG frame (1MB)
3. **corrupted_frame.bin** - Corrupted data
4. **test_config.yaml** - Test configuration
5. **high_fps_config.yaml** - 10 cameras @ 30 FPS config

---

## 10. CONTINUOUS TESTING

### GitHub Actions Workflow

```yaml
name: QA Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
      - name: Run Unit Tests
        run: go test ./internal/... -v -cover
      - name: Run Integration Tests
        run: go test ./tests/integration/... -v
```

---

## SUMMARY

**Total Tests:** 25+
**Estimated Coverage:** 85%+
**Execution Time:** ~90 minutes (full suite)

**Priority Order:**
1. Unit Tests (critical path)
2. Integration Tests (core functionality)
3. Stress Tests (stability)
4. E2E Tests (user experience)
5. Benchmarks (optimization)
