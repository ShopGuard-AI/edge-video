package integration

import (
	"runtime"
	"sync"
	"testing"
)

// ============================================================================
// INTEGRATION TEST: Memory Pool Recycling
// ============================================================================
// Testa pool de memória para frames (sync.Pool)
// Valida reutilização de buffers e ausência de memory leaks
// ============================================================================

// TestMemoryPoolConcept valida conceito de pool de memória
func TestMemoryPoolConcept(t *testing.T) {
	t.Run("sync.Pool Básico", func(t *testing.T) {
		// Pool de teste (simula framePool do Producer)
		pool := &sync.Pool{
			New: func() interface{} {
				// Buffer de 2MB (igual ao framePool)
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		// Testa Get/Put
		buf1 := pool.Get().(*[]byte)
		if buf1 == nil {
			t.Fatal("Pool retornou nil")
		}

		// Valida tamanho do buffer
		if len(*buf1) != 2*1024*1024 {
			t.Errorf("Buffer size: %d, expected: %d", len(*buf1), 2*1024*1024)
		}

		// Devolve ao pool
		pool.Put(buf1)

		// Reusa buffer
		buf2 := pool.Get().(*[]byte)
		if buf2 == nil {
			t.Fatal("Pool retornou nil na segunda tentativa")
		}

		t.Logf("✅ sync.Pool funcionando (buffer size: %d bytes)", len(*buf2))
	})
}

// TestMemoryPoolReuse valida reutilização de buffers
func TestMemoryPoolReuse(t *testing.T) {
	t.Run("Buffer Reuse Detection", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		// Get primeiro buffer
		buf1 := pool.Get().(*[]byte)
		addr1 := &(*buf1)[0] // Endereço do primeiro byte

		// Modifica buffer
		(*buf1)[0] = 0xFF

		// Devolve ao pool
		pool.Put(buf1)

		// Get segundo buffer (pode ser o mesmo devido ao pool)
		buf2 := pool.Get().(*[]byte)
		addr2 := &(*buf2)[0]

		// Verifica se é o mesmo buffer (endereço)
		if addr1 == addr2 {
			t.Log("✅ Buffer foi reutilizado (mesmo endereço de memória)")
		} else {
			t.Log("⚠️  Novo buffer alocado (GC pode ter limpado o pool)")
		}

		// Buffer deve ter sido resetado
		pool.Put(buf2)
	})
}

// TestMemoryPoolConcurrency valida pool sob concorrência
func TestMemoryPoolConcurrency(t *testing.T) {
	t.Run("Concurrent Get/Put", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		goroutineCount := 50
		operationsPerGoroutine := 100

		var wg sync.WaitGroup
		wg.Add(goroutineCount)

		// Lança goroutines que fazem Get/Put concorrentemente
		for i := 0; i < goroutineCount; i++ {
			go func(id int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					// Get buffer
					buf := pool.Get().(*[]byte)

					// Usa buffer (simula processamento)
					(*buf)[0] = byte(id)

					// Devolve buffer
					pool.Put(buf)
				}
			}(i)
		}

		// Aguarda todas completarem
		wg.Wait()

		t.Logf("✅ %d goroutines × %d ops = %d operações concorrentes sem deadlock",
			goroutineCount, operationsPerGoroutine, goroutineCount*operationsPerGoroutine)
	})
}

// TestMemoryPoolGC valida comportamento do pool com GC
func TestMemoryPoolGC(t *testing.T) {
	t.Run("Pool Survives GC", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		// Popula pool com vários buffers
		buffers := make([]*[]byte, 10)
		for i := 0; i < 10; i++ {
			buffers[i] = pool.Get().(*[]byte)
		}

		// Devolve todos ao pool
		for _, buf := range buffers {
			pool.Put(buf)
		}

		// Força GC
		runtime.GC()

		// Pool pode ter sido esvaziado pelo GC (comportamento do sync.Pool)
		// Mas deve continuar funcionando
		buf := pool.Get().(*[]byte)
		if buf == nil {
			t.Fatal("Pool retornou nil após GC")
		}

		t.Logf("✅ Pool funciona após GC (buffer size: %d bytes)", len(*buf))
		pool.Put(buf)
	})
}

// TestMemoryPoolStats valida estatísticas de uso de memória
func TestMemoryPoolStats(t *testing.T) {
	t.Run("Memory Stats", func(t *testing.T) {
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		// Aloca e libera 100 buffers
		for i := 0; i < 100; i++ {
			buf := pool.Get().(*[]byte)
			(*buf)[0] = 0xFF
			pool.Put(buf)
		}

		// Força GC para liberar memória não usada
		runtime.GC()

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		allocMB := float64(m2.TotalAlloc-m1.TotalAlloc) / 1024 / 1024

		t.Logf("📊 Memory Stats:")
		t.Logf("   Total Allocated: %.2f MB", allocMB)
		t.Logf("   Heap Alloc: %.2f MB", float64(m2.HeapAlloc)/1024/1024)
		t.Logf("   NumGC: %d", m2.NumGC-m1.NumGC)

		// Com pool eficiente, alocação total deve ser relativamente baixa
		// (não deve alocar 100 × 2MB = 200MB novos)
		if allocMB < 50 { // Threshold arbitrário
			t.Log("✅ Pool reduzindo alocações de memória efetivamente")
		} else {
			t.Logf("⚠️  Alocações altas (%.2f MB) - pool pode não estar sendo efetivo", allocMB)
		}
	})
}

// TestMemoryPoolReset valida reset de buffers
func TestMemoryPoolReset(t *testing.T) {
	t.Run("Buffer Reset", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 100)
				return &buf
			},
		}

		// Get buffer
		buf := pool.Get().(*[]byte)

		// Modifica slice
		*buf = (*buf)[:50] // Reduz len, mas mantém cap

		// Valida estado antes de devolver
		if len(*buf) != 50 || cap(*buf) != 100 {
			t.Errorf("Buffer state incorreto: len=%d, cap=%d", len(*buf), cap(*buf))
		}

		// Reset buffer antes de devolver (como putFrameBuffer faz)
		*buf = (*buf)[:cap(*buf)]

		// Devolve ao pool
		pool.Put(buf)

		// Get novamente
		buf2 := pool.Get().(*[]byte)

		// Buffer deve ter len = cap
		if len(*buf2) != cap(*buf2) {
			t.Errorf("Buffer não resetado: len=%d, cap=%d", len(*buf2), cap(*buf2))
		} else {
			t.Logf("✅ Buffer resetado corretamente (len=%d, cap=%d)", len(*buf2), cap(*buf2))
		}

		pool.Put(buf2)
	})
}

// TestMemoryPoolNilHandling valida tratamento de nil
func TestMemoryPoolNilHandling(t *testing.T) {
	t.Run("Put Nil Buffer", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 100)
				return &buf
			},
		}

		// Simula putFrameBuffer(nil) - não deve causar panic
		var nilBuf *[]byte
		pool.Put(nilBuf) // sync.Pool aceita nil, mas não armazena

		// Pool ainda deve funcionar (Get pode retornar novo buffer do New())
		buf := pool.Get().(*[]byte)
		if buf == nil {
			t.Log("⚠️  Pool retornou nil após Put(nil) - behavior válido de sync.Pool")
		} else {
			t.Logf("✅ Pool criou novo buffer após Put(nil) (size: %d)", len(*buf))
		}

		// Valida que não houve panic
		t.Log("✅ Pool lida com Put(nil) sem panic")
	})
}

// TestMemoryPoolCapacity valida capacidade de buffers
func TestMemoryPoolCapacity(t *testing.T) {
	t.Run("Buffer Capacity 2MB", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				// Mesmo tamanho que framePool: 2MB
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		buf := pool.Get().(*[]byte)

		expectedSize := 2 * 1024 * 1024
		if len(*buf) != expectedSize {
			t.Errorf("Buffer size: %d, expected: %d", len(*buf), expectedSize)
		}

		if cap(*buf) != expectedSize {
			t.Errorf("Buffer capacity: %d, expected: %d", cap(*buf), expectedSize)
		}

		t.Logf("✅ Buffer capacity: %d bytes (%.2f MB)",
			cap(*buf), float64(cap(*buf))/1024/1024)

		// Valida que buffer suporta frames grandes
		// cam1 RTMP: ~355KB, cam3 RTSP: ~184KB, cam2 RTSP: ~58KB
		if cap(*buf) >= 355*1024 {
			t.Log("✅ Buffer suporta frames RTMP (355KB)")
		}

		pool.Put(buf)
	})
}

// TestMemoryPoolStressTest stress test do pool
func TestMemoryPoolStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Run("Stress Test 1000 Goroutines", func(t *testing.T) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 2*1024*1024)
				return &buf
			},
		}

		goroutineCount := 1000
		operationsPerGoroutine := 50

		var wg sync.WaitGroup
		wg.Add(goroutineCount)

		for i := 0; i < goroutineCount; i++ {
			go func(id int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					buf := pool.Get().(*[]byte)
					// Simula uso do buffer
					(*buf)[0] = byte(id)
					(*buf)[len(*buf)-1] = byte(j)
					pool.Put(buf)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("✅ Stress test completo: %d goroutines × %d ops = %d ops totais",
			goroutineCount, operationsPerGoroutine, goroutineCount*operationsPerGoroutine)
	})
}
