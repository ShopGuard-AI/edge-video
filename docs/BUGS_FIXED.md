# Edge Video V2 - Bugs Críticos Corrigidos

## Bug #1: Frame Cross-Contamination (V2.1)

**Sintoma**: Frames de cam2 apareciam no viewer cam1 (5-15%).
**Impacto**: CRÍTICO - Violação isolamento dados.

**Causa**: `sync.Pool` GLOBAL compartilhado entre câmeras.

**Solução**:
```go
// ANTES (bugado) - pool.go
var framePool = &sync.Pool{...}  // GLOBAL!

// DEPOIS (correto) - camera_stream.go
type CameraStream struct {
    pool *sync.Pool  // LOCAL por câmera
}
```

**Tentativas falhadas antes da solução**: 6
1. Validação routing keys
2. Mutex publisher
3. Publishers dedicados
4. Defensive copy
5. Copy imediato
6. Troca lib AMQP
7. **Análise forense → RESOLVIDO**

---

## Bug #2: Goroutine Leak em Publisher (V2.2)

**Sintoma**: Goroutines acumulam após reconexões RabbitMQ.
**Impacto**: ALTO - Crash OOM produção.

**Causa**: `connect()` cria `handleConfirms()` sem parar anterior.

**Solução**:
```go
type Publisher struct {
    confirmsDone chan struct{}  // Sinal parada
}

func (p *Publisher) connect() {
    // Para goroutine antigo
    if p.confirmsDone != nil {
        close(p.confirmsDone)
    }
    p.confirmsDone = make(chan struct{})
    go p.handleConfirms()
}

func (p *Publisher) handleConfirms() {
    for {
        select {
        case <-p.confirmsDone:  // NOVO
            return
        case confirm := <-p.confirmsChan:
            // Processa
        }
    }
}
```

**Teste**: 5 reconexões
- ANTES: 5 goroutines leaked
- DEPOIS: 0 goroutines leaked

---

## Bug #3: JPEG Decode Falha (46.9%)

**Sintoma**: OpenCV falha 46.9% dos JPEGs RabbitMQ.
**Status**: Diagnosticado (frames corrompidos/truncados).

**Teste offline**: PIL/OpenCV decodificam OK.
**Problema**: Tempo real (RabbitMQ → Consumer).

**Causas possíveis**:
1. Perda pacotes rede
2. Race condition publisher
3. Limite RabbitMQ truncando
4. FFmpeg frames malformados

**Solução proposta**:
```python
img = cv2.imdecode(np_arr, cv2.IMREAD_COLOR)
if img is None:  # Fallback PIL
    img_pil = Image.open(io.BytesIO(frame_bytes))
    img = cv2.cvtColor(np.array(img_pil), cv2.COLOR_RGB2BGR)
```

---

## Melhorias V2.2+

- ✅ Circuit Breaker backoff exponencial
- ✅ Memory Controller RAM enforcement
- ✅ System metrics (CPU/RAM)
- ✅ Publisher confirms (ACK/NACK 100%)
- ✅ Auto-reconnect AMQP
- ✅ Latest Frame Policy (0% drops)
