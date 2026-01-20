package messaging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"edge-video/v2/internal/monitoring"
	"edge-video/v2/internal/storage"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher gerencia publicação no RabbitMQ com auto-reconnect
type Publisher struct {
	amqpURL       string
	conn          *amqp.Connection
	channel       *amqp.Channel
	exchange      string
	routingKey    string // Routing key COMPLETA (não é mais prefixo)
	prefetchCount int    // QoS: limite de frames não-confirmados (0 = ilimitado)

	// Redis client (opcional - se habilitado, armazena frames)
	redisClient *storage.RedisClient

	// Publisher Confirms (configurável - DEVE SER FALSE se não houver consumer!)
	publisherConfirmsEnabled bool

	mu            sync.Mutex
	// REMOVIDO publishMu - amqp091-go channel.Publish() é thread-safe!
	publishCount  uint64
	publishErrors uint64
	reconnecting  bool
	connected     bool

	// Publisher Confirms (rastreamento de entregas)
	confirmsChan     chan amqp.Confirmation
	confirmsCount    uint64 // Total de confirms recebidos (ACK)
	nacksCount       uint64 // Total de NACKs recebidos (rejeições)
	confirmsDone     chan struct{} // Canal para sinalizar fim do handleConfirms (evita goroutine leak)

	notifyClose chan *amqp.Error
	done        chan struct{}
}

// NewPublisher cria um novo publisher com auto-reconnect
func NewPublisher(amqpURL, exchange, routingKey string, prefetchCount int, publisherConfirms bool, redisClient *storage.RedisClient) (*Publisher, error) {
	p := &Publisher{
		amqpURL:                  amqpURL,
		exchange:                 exchange,
		routingKey:               routingKey,         // Usa routing_key completa
		prefetchCount:            prefetchCount,      // QoS configurável
		publisherConfirmsEnabled: publisherConfirms,  // Publisher Confirms configurável
		redisClient:              redisClient,        // Redis client (pode ser nil se disabled)
		done:                     make(chan struct{}),
	}

	// Conecta inicialmente com retry
	err := p.connectWithRetry(10, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Monitora conexão em background
	go p.monitorConnection()

	log.Printf("✓ Conectado ao RabbitMQ - Exchange: %s", exchange)

	return p, nil
}

// connectWithRetry tenta conectar com retry exponencial
func (p *Publisher) connectWithRetry(maxRetries int, initialDelay time.Duration) error {
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err := p.connect()
		if err == nil {
			p.mu.Lock()
			p.connected = true
			p.mu.Unlock()
			return nil
		}

		log.Printf("⚠ Tentativa %d/%d falhou: %v. Retry em %v...", i+1, maxRetries, err, delay)
		time.Sleep(delay)

		// Backoff exponencial: 5s, 10s, 20s (max 30s)
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}

	return fmt.Errorf("falha após %d tentativas", maxRetries)
}

// connect estabelece conexão com RabbitMQ
func (p *Publisher) connect() error {
	var err error

	// CRITICAL FIX: Para o goroutine handleConfirms anterior ANTES de criar um novo
	// Isso previne goroutine leak durante reconexões
	if p.confirmsDone != nil {
		close(p.confirmsDone)  // Sinaliza para o goroutine anterior parar
		p.confirmsDone = nil   // Limpa referência
		time.Sleep(10 * time.Millisecond)  // Aguarda goroutine anterior encerrar
	}

	// Conecta
	p.conn, err = amqp.Dial(p.amqpURL)
	if err != nil {
		return fmt.Errorf("falha ao conectar: %w", err)
	}

	// Cria canal
	p.channel, err = p.conn.Channel()
	if err != nil {
		p.conn.Close()
		return fmt.Errorf("falha ao criar canal: %w", err)
	}

	// Declara exchange APENAS se não for o default exchange
	// Default exchange ("") é built-in do RabbitMQ e não pode ser declarado
	if p.exchange != "" {
		err = p.channel.ExchangeDeclare(
			p.exchange,
			"topic",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			p.channel.Close()
			p.conn.Close()
			return fmt.Errorf("falha ao declarar exchange: %w", err)
		}
		log.Printf("✓ Exchange declarado: %s (type: topic, durable: true)", p.exchange)
	} else {
		log.Printf("⚠️  Usando default exchange (direto para queue via routing_key)")
	}

	// CONFIGURA QoS (Quality of Service)
	// Limita quantos frames não-confirmados podem estar em trânsito
	// Isso previne:
	// - Consumer overflow (consumer recebe milhares de frames de uma vez)
	// - Memory overflow no consumer
	// - Processamento em lote que causa latência
	err = p.channel.Qos(
		p.prefetchCount, // prefetchCount: configurável via config.yaml (0 = ilimitado)
		0,               // prefetchSize: sem limite de bytes (0 = ilimitado)
		false,           // global: false = aplica apenas a este channel
	)
	if err != nil {
		p.channel.Close()
		p.conn.Close()
		return fmt.Errorf("falha ao configurar QoS: %w", err)
	}

	// PUBLISHER CONFIRMS (condicional - depende da configuração)
	if p.publisherConfirmsEnabled {
		// Habilita Publisher Confirms
		// Isso faz o RabbitMQ enviar confirmações (ACK/NACK) para cada mensagem publicada
		err = p.channel.Confirm(false)
		if err != nil {
			p.channel.Close()
			p.conn.Close()
			return fmt.Errorf("falha ao habilitar publisher confirms: %w", err)
		}

		// Canal para receber confirmações
		p.confirmsChan = p.channel.NotifyPublish(make(chan amqp.Confirmation, 1000))

		// Cria novo canal de controle para este goroutine
		p.confirmsDone = make(chan struct{})

		// Inicia goroutine para processar confirmações
		go p.handleConfirms()

		log.Printf("✓ QoS configurado: prefetch=%d | Publisher Confirms HABILITADO para exchange: %s",
			p.prefetchCount, p.exchange)
	} else {
		log.Printf("✓ QoS configurado: prefetch=%d | Publisher Confirms DESABILITADO para exchange: %s",
			p.prefetchCount, p.exchange)
	}

	// Monitora fechamento de conexão
	p.notifyClose = make(chan *amqp.Error)
	p.conn.NotifyClose(p.notifyClose)

	return nil
}

// handleConfirms processa confirmações (ACK/NACK) do RabbitMQ
func (p *Publisher) handleConfirms() {
	for {
		select {
		case <-p.done:
			// Publisher.Close() foi chamado
			return

		case <-p.confirmsDone:
			// Reconexão em andamento - para este goroutine para evitar leak
			return

		case confirm, ok := <-p.confirmsChan:
			if !ok {
				// Canal fechado (reconexão em andamento)
				return
			}

			p.mu.Lock()
			if confirm.Ack {
				// ACK: Frame entregue com sucesso ao RabbitMQ
				p.confirmsCount++
			} else {
				// NACK: Frame rejeitado pelo RabbitMQ
				p.nacksCount++
				log.Printf("⚠️  NACK recebido! Frame rejeitado pelo RabbitMQ (delivery tag: %d)", confirm.DeliveryTag)
			}
			p.mu.Unlock()

			// Tracking para profiling
			monitoring.TrackPublishConfirm(confirm.Ack)
		}
	}
}

// monitorConnection monitora e reconecta automaticamente
func (p *Publisher) monitorConnection() {
	for {
		select {
		case <-p.done:
			return

		case err := <-p.notifyClose:
			if err != nil {
				log.Printf("🛑 Conexão RabbitMQ perdida: %v", err)
				p.mu.Lock()
				p.connected = false
				p.mu.Unlock()

				p.reconnect()
			}
		}
	}
}

// reconnect tenta reconectar indefinidamente
func (p *Publisher) reconnect() {
	p.mu.Lock()
	if p.reconnecting {
		p.mu.Unlock()
		return
	}
	p.reconnecting = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.reconnecting = false
		p.mu.Unlock()
	}()

	delay := 1 * time.Second

	for {
		select {
		case <-p.done:
			return
		default:
		}

		log.Printf("🔄 Tentando reconectar ao RabbitMQ...")

		// Fecha conexão antiga se existir
		if p.channel != nil {
			p.channel.Close()
		}
		if p.conn != nil {
			p.conn.Close()
		}

		// Tenta reconectar
		err := p.connect()
		if err == nil {
			p.mu.Lock()
			p.connected = true
			p.mu.Unlock()
			log.Printf("✓ Reconectado ao RabbitMQ com sucesso!")
			return
		}

		log.Printf("⚠ Reconexão falhou: %v. Retry em %v...", err, delay)
		time.Sleep(delay)

		// Backoff exponencial: 1s, 2s, 5s, 10s (max 10s)
		if delay < 2*time.Second {
			delay = 2 * time.Second
		} else if delay < 5*time.Second {
			delay = 5 * time.Second
		} else {
			delay = 10 * time.Second
		}
	}
}

// Publish publica um frame no RabbitMQ com retry (OTIMIZADO - SEM MUTEX GLOBAL!)
func (p *Publisher) Publish(cameraID string, frameData []byte, timestamp time.Time) error {
	startTotal := time.Now()

	// 1. Verifica conexão (lock mínimo)
	p.mu.Lock()
	if !p.connected {
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("não conectado ao RabbitMQ")
	}
	routingKey := p.routingKey
	channel := p.channel
	shouldDebug := p.publishCount < 18
	p.mu.Unlock()

	// DEBUG: Log detalhado de publicação (primeiros 18 frames)
	if shouldDebug {
		log.Printf("[PUBLISH DEBUG] Camera: %s, RoutingKey: %s, Size: %d bytes, Header[camera_id]: %s",
			cameraID, routingKey, len(frameData), cameraID)
	}

	// 2. CÓPIA DEFENSIVA (fora de qualquer lock)
	frameDataCopy := make([]byte, len(frameData))
	copy(frameDataCopy, frameData)

	// 3. REDIS STORE (OBRIGATÓRIO - sem Redis, não há como enviar frame!)
	// V1.6-STYLE: Redis é MANDATÓRIO pois apenas redis_key é enviado ao RabbitMQ
	var redisKey string
	var redisTime time.Duration
	if p.redisClient == nil || !p.redisClient.IsEnabled() {
		p.mu.Lock()
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("Redis OBRIGATÓRIO para V1.6-style publishing (redis_enabled=false no config)")
	}

	startRedis := time.Now()
	var storeErr error
	redisKey, storeErr = p.redisClient.Store(cameraID, frameDataCopy, timestamp)
	redisTime = time.Since(startRedis)

	if storeErr != nil {
		p.mu.Lock()
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("[%s] Redis store FALHOU (obrigatório): %w", cameraID, storeErr)
	}

	// 4. RABBITMQ PUBLISH (PARALELO - channel.Publish() é thread-safe!)
	// V1.6-STYLE: Envia apenas redis_key no body, NÃO o frame completo
	// Isso reduz drasticamente o tamanho da mensagem (316KB → ~50 bytes)

	// Calcula TTL da mensagem (mesmo TTL do Redis para consistência)
	var ttlMs string
	if p.redisClient != nil {
		ttlMs = fmt.Sprintf("%d", int64(p.redisClient.GetTTL().Milliseconds()))
	}

	startRabbit := time.Now()
	err := channel.Publish(
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte(redisKey), // Apenas redis_key (ex: "frames:cam1:1733681234567")
			Timestamp:    timestamp,
			DeliveryMode: amqp.Transient, // Não persiste (mais rápido)
			Expiration:   ttlMs,          // TTL da mensagem (igual ao Redis: 120s = 120000ms)
			Headers: amqp.Table{
				"camera_id": cameraID,
				"redis_key": redisKey,
			},
		},
	)
	rabbitTime := time.Since(startRabbit)
	totalTime := time.Since(startTotal)

	// 5. Atualiza contadores (lock mínimo)
	p.mu.Lock()
	if err != nil {
		p.publishErrors++
		p.connected = false
		p.mu.Unlock()

		// Trigger reconexão
		go p.reconnect()

		return fmt.Errorf("falha ao publicar: %w", err)
	}

	p.publishCount++
	count := p.publishCount
	p.mu.Unlock()

	// 6. Log de timing detalhado a cada 100 frames
	if count%100 == 0 {
		log.Printf("[%s] Frame #%d | Total: %v | Redis: %v | RabbitMQ: %v",
			cameraID, count, totalTime, redisTime, rabbitTime)
	}

	return nil
}

// PublishWithContext publica um frame no RabbitMQ com context awareness
// Se o contexto for cancelado, retorna imediatamente sem tentar publish
func (p *Publisher) PublishWithContext(ctx context.Context, cameraID string, frameData []byte, timestamp time.Time) error {
	// Verifica se contexto já foi cancelado antes de começar
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	startTotal := time.Now()

	// 1. Verifica conexão (lock mínimo)
	p.mu.Lock()
	if !p.connected {
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("não conectado ao RabbitMQ")
	}
	routingKey := p.routingKey
	channel := p.channel
	shouldDebug := p.publishCount < 18
	p.mu.Unlock()

	// DEBUG: Log detalhado de publicação (primeiros 18 frames)
	if shouldDebug {
		log.Printf("[PUBLISH DEBUG] Camera: %s, RoutingKey: %s, Size: %d bytes, Header[camera_id]: %s",
			cameraID, routingKey, len(frameData), cameraID)
	}

	// Verifica contexto antes de operações pesadas
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 2. CÓPIA DEFENSIVA (fora de qualquer lock)
	frameDataCopy := make([]byte, len(frameData))
	copy(frameDataCopy, frameData)

	// 3. REDIS STORE (OBRIGATÓRIO - com context awareness!)
	// V1.6-STYLE: Redis é MANDATÓRIO pois apenas redis_key é enviado ao RabbitMQ
	var redisKey string
	var redisTime time.Duration
	if p.redisClient == nil || !p.redisClient.IsEnabled() {
		p.mu.Lock()
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("Redis OBRIGATÓRIO para V1.6-style publishing (redis_enabled=false no config)")
	}

	startRedis := time.Now()
	var storeErr error
	// ✅ USA STOREWITCONTEXT para respeitar cancelamento!
	redisKey, storeErr = p.redisClient.StoreWithContext(ctx, cameraID, frameDataCopy, timestamp)
	redisTime = time.Since(startRedis)

	if storeErr != nil {
		// Se falhou por cancelamento de contexto, retorna ctx.Err()
		if ctx.Err() != nil {
			return ctx.Err()
		}

		p.mu.Lock()
		p.publishErrors++
		p.mu.Unlock()
		return fmt.Errorf("[%s] Redis store FALHOU (obrigatório): %w", cameraID, storeErr)
	}

	// Verifica contexto antes de RabbitMQ publish
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 4. RABBITMQ PUBLISH (PARALELO - channel.Publish() é thread-safe!)
	// V1.6-STYLE: Envia apenas redis_key no body, NÃO o frame completo
	// Isso reduz drasticamente o tamanho da mensagem (316KB → ~50 bytes)

	// Calcula TTL da mensagem (mesmo TTL do Redis para consistência)
	var ttlMs string
	if p.redisClient != nil {
		ttlMs = fmt.Sprintf("%d", int64(p.redisClient.GetTTL().Milliseconds()))
	}

	startRabbit := time.Now()
	err := channel.Publish(
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte(redisKey), // Apenas redis_key (ex: "frames:cam1:1733681234567")
			Timestamp:    timestamp,
			DeliveryMode: amqp.Transient, // Não persiste (mais rápido)
			Expiration:   ttlMs,          // TTL da mensagem (igual ao Redis: 120s = 120000ms)
			Headers: amqp.Table{
				"camera_id": cameraID,
				"redis_key": redisKey,
			},
		},
	)
	rabbitTime := time.Since(startRabbit)
	totalTime := time.Since(startTotal)

	// 5. Atualiza contadores (lock mínimo)
	p.mu.Lock()
	if err != nil {
		p.publishErrors++
		p.connected = false
		p.mu.Unlock()

		// Trigger reconexão
		go p.reconnect()

		return fmt.Errorf("falha ao publicar: %w", err)
	}

	p.publishCount++
	count := p.publishCount
	p.mu.Unlock()

	// 6. Log de timing detalhado a cada 100 frames
	if count%100 == 0 {
		log.Printf("[%s] Frame #%d | Total: %v | Redis: %v | RabbitMQ: %v",
			cameraID, count, totalTime, redisTime, rabbitTime)
	}

	return nil
}

// Close fecha a conexão
func (p *Publisher) Close() error {
	close(p.done)

	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// Stats retorna estatísticas
func (p *Publisher) Stats() (uint64, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.publishCount, p.publishErrors
}

// IsConnected retorna se está conectado
func (p *Publisher) IsConnected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected
}

// ConfirmStats retorna estatísticas de confirmações
func (p *Publisher) ConfirmStats() (acks uint64, nacks uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.confirmsCount, p.nacksCount
}
