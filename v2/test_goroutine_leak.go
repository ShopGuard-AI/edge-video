package main

import (
	"fmt"
	"log"
	"runtime"
	"time"
)

// Simula o problema do goroutine leak no Publisher

type FakePublisher struct {
	done         chan struct{}
	confirmsChan chan struct{}
}

func NewFakePublisher() *FakePublisher {
	return &FakePublisher{
		done: make(chan struct{}),
	}
}

// Simula connect() - CRIA NOVO GOROUTINE a cada call
func (p *FakePublisher) connect() {
	p.confirmsChan = make(chan struct{}, 10)

	// PROBLEMA: Cria novo goroutine sem parar o anterior!
	go p.handleConfirms()

	log.Println("✓ Conectado (novo goroutine handleConfirms criado)")
}

// Simula handleConfirms() - goroutine que processa confirmações
func (p *FakePublisher) handleConfirms() {
	log.Println("  [GOROUTINE] handleConfirms INICIADO")
	for {
		select {
		case <-p.done:
			log.Println("  [GOROUTINE] handleConfirms ENCERRADO (via done)")
			return
		case _, ok := <-p.confirmsChan:
			if !ok {
				log.Println("  [GOROUTINE] handleConfirms ENCERRADO (channel fechado)")
				return
			}
		}
	}
}

// Simula reconnect() - chama connect() novamente
func (p *FakePublisher) reconnect() {
	log.Println("🔄 Reconectando...")
	// PROBLEMA: Chama connect() que cria OUTRO goroutine!
	p.connect()
}

func (p *FakePublisher) Close() {
	close(p.done)
}

func main() {
	log.Println("========================================")
	log.Println("TESTE DE GOROUTINE LEAK")
	log.Println("========================================\n")

	// Conta goroutines iniciais
	initialGoroutines := runtime.NumGoroutine()
	log.Printf("Goroutines INICIAIS: %d\n", initialGoroutines)

	pub := NewFakePublisher()

	// Conexão inicial
	log.Println("\n--- CONEXÃO INICIAL ---")
	pub.connect()
	time.Sleep(100 * time.Millisecond)
	log.Printf("Goroutines após 1ª conexão: %d (esperado: %d, atual: %d)\n",
		runtime.NumGoroutine(), initialGoroutines+1, runtime.NumGoroutine())

	// Simula 5 reconexões
	for i := 1; i <= 5; i++ {
		log.Printf("\n--- RECONEXÃO #%d ---\n", i)
		pub.reconnect()
		time.Sleep(100 * time.Millisecond)

		actual := runtime.NumGoroutine()

		if actual > initialGoroutines + 1 {
			log.Printf("⚠️  LEAK DETECTADO! Goroutines: %d (esperado: %d, LEAK: +%d)\n",
				actual, initialGoroutines+1, actual - (initialGoroutines+1))
		} else {
			log.Printf("✓ Goroutines: %d (sem leak)\n", actual)
		}
	}

	// Relatório final
	log.Println("\n========================================")
	log.Println("RELATÓRIO FINAL")
	log.Println("========================================")

	finalGoroutines := runtime.NumGoroutine()
	expectedGoroutines := initialGoroutines + 1  // Apenas 1 handleConfirms deveria estar rodando
	leak := finalGoroutines - expectedGoroutines

	log.Printf("Goroutines INICIAIS:    %d\n", initialGoroutines)
	log.Printf("Goroutines ESPERADOS:   %d (inicial + 1 handleConfirms)\n", expectedGoroutines)
	log.Printf("Goroutines ATUAIS:      %d\n", finalGoroutines)
	log.Printf("GOROUTINES LEAKED:      %d\n", leak)

	if leak > 0 {
		fmt.Printf("\n🔴 GOROUTINE LEAK CONFIRMADO!\n")
		fmt.Printf("   - %d reconexões criaram %d goroutines órfãos\n", 5, leak)
		fmt.Printf("   - Cada reconexão deveria PARAR o goroutine anterior antes de criar novo\n")
		fmt.Printf("   - Em produção, isso causa acúmulo de goroutines até crash!\n")
	} else {
		fmt.Printf("\n✅ Nenhum leak detectado\n")
	}

	pub.Close()
	time.Sleep(200 * time.Millisecond)  // Aguarda goroutines encerrarem

	log.Printf("\nGoroutines após Close(): %d\n", runtime.NumGoroutine())
	log.Println("\n========================================")
}
