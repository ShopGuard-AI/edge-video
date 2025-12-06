package main

import (
	"fmt"
	"log"
	"runtime"
	"time"
)

// Simula o problema do goroutine leak no Publisher - VERSÃO CORRIGIDA

type FakePublisherFixed struct {
	done         chan struct{}
	confirmsChan chan struct{}
	confirmsDone chan struct{} // NOVA: Canal para sinalizar fim do handleConfirms
}

func NewFakePublisherFixed() *FakePublisherFixed {
	return &FakePublisherFixed{
		done: make(chan struct{}),
	}
}

// Simula connect() - CORRIGIDO: Para goroutine anterior antes de criar novo
func (p *FakePublisherFixed) connect() {
	// FIX: Para o goroutine handleConfirms anterior ANTES de criar um novo
	if p.confirmsDone != nil {
		close(p.confirmsDone)  // Sinaliza para o goroutine anterior parar
		p.confirmsDone = nil
		time.Sleep(10 * time.Millisecond)  // Aguarda goroutine anterior encerrar
	}

	p.confirmsChan = make(chan struct{}, 10)
	p.confirmsDone = make(chan struct{})  // Novo canal de controle

	// AGORA: Cria novo goroutine (mas parou o anterior primeiro!)
	go p.handleConfirms()

	log.Println("✓ Conectado (novo goroutine handleConfirms criado, anterior foi parado)")
}

// Simula handleConfirms() - CORRIGIDO: Também escuta confirmsDone
func (p *FakePublisherFixed) handleConfirms() {
	log.Println("  [GOROUTINE] handleConfirms INICIADO")
	for {
		select {
		case <-p.done:
			log.Println("  [GOROUTINE] handleConfirms ENCERRADO (via done)")
			return

		case <-p.confirmsDone:
			// FIX: Novo canal para parar durante reconexão
			log.Println("  [GOROUTINE] handleConfirms ENCERRADO (via confirmsDone - reconexão)")
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
func (p *FakePublisherFixed) reconnect() {
	log.Println("🔄 Reconectando...")
	// Agora connect() para o goroutine anterior primeiro!
	p.connect()
}

func (p *FakePublisherFixed) Close() {
	close(p.done)
}

func main() {
	log.Println("========================================")
	log.Println("TESTE DE GOROUTINE LEAK - VERSÃO CORRIGIDA")
	log.Println("========================================\n")

	// Conta goroutines iniciais
	initialGoroutines := runtime.NumGoroutine()
	log.Printf("Goroutines INICIAIS: %d\n", initialGoroutines)

	pub := NewFakePublisherFixed()

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
			log.Printf("✅ Goroutines: %d (sem leak!)\n", actual)
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
		fmt.Printf("\n🔴 GOROUTINE LEAK AINDA EXISTE!\n")
		fmt.Printf("   - %d reconexões criaram %d goroutines órfãos\n", 5, leak)
		fmt.Printf("   - Fix NÃO funcionou corretamente!\n")
	} else {
		fmt.Printf("\n✅ GOROUTINE LEAK CORRIGIDO COM SUCESSO!\n")
		fmt.Printf("   - 5 reconexões realizadas\n")
		fmt.Printf("   - 0 goroutines leaked (antigos foram parados corretamente)\n")
		fmt.Printf("   - Apenas 1 handleConfirms ativo (o mais recente)\n")
		fmt.Printf("   - Solução: Cada reconexão para o goroutine anterior via confirmsDone\n")
	}

	pub.Close()
	time.Sleep(200 * time.Millisecond)  // Aguarda goroutines encerrarem

	log.Printf("\nGoroutines após Close(): %d\n", runtime.NumGoroutine())
	log.Println("\n========================================")
}
