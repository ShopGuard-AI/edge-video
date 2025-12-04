package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// Publisher gerencia publicação no RabbitMQ
type Publisher struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	exchange         string
	routingKeyPrefix string

	mu             sync.Mutex
	publishCount   uint64
	publishErrors  uint64
	reconnecting   bool
}

// NewPublisher cria um novo publisher
func NewPublisher(amqpURL, exchange, routingKeyPrefix string) (*Publisher, error) {
	p := &Publisher{
		exchange:         exchange,
		routingKeyPrefix: routingKeyPrefix,
	}

	err := p.connect(amqpURL)
	if err != nil {
		return nil, err
	}

	log.Printf("Conectado ao RabbitMQ - Exchange: %s", exchange)

	return p, nil
}

// connect estabelece conexão com RabbitMQ
func (p *Publisher) connect(amqpURL string) error {
	var err error

	// Conecta
	p.conn, err = amqp.Dial(amqpURL)
	if err != nil {
		return fmt.Errorf("falha ao conectar: %w", err)
	}

	// Cria canal
	p.channel, err = p.conn.Channel()
	if err != nil {
		p.conn.Close()
		return fmt.Errorf("falha ao criar canal: %w", err)
	}

	// Declara exchange
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

	return nil
}

// Publish publica um frame no RabbitMQ
func (p *Publisher) Publish(cameraID string, frameData []byte, timestamp time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	routingKey := p.routingKeyPrefix + cameraID

	err := p.channel.Publish(
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/octet-stream",
			Body:         frameData,
			Timestamp:    timestamp,
			DeliveryMode: amqp.Transient, // Não persiste (mais rápido)
		},
	)

	if err != nil {
		p.publishErrors++
		return fmt.Errorf("falha ao publicar: %w", err)
	}

	p.publishCount++
	return nil
}

// Close fecha a conexão
func (p *Publisher) Close() error {
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
