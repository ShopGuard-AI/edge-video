package mq

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/streadway/amqp"
)

type AMQPPublisher struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	exchange         string
	routingKeyPrefix string
	amqpURL          string
}

func NewAMQPPublisher(amqpURL, exchange, routingKeyPrefix string) (*AMQPPublisher, error) {
	publisher := &AMQPPublisher{
		exchange:         exchange,
		routingKeyPrefix: routingKeyPrefix,
		amqpURL:          amqpURL,
	}

	// Tenta conectar com retry
	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		err = publisher.connect()
		if err == nil {
			log.Printf("Conectado ao RabbitMQ com sucesso")
			return publisher, nil
		}
		log.Printf("Tentativa %d/%d de conexão ao RabbitMQ falhou: %v. Tentando novamente em 5s...", i+1, maxRetries, err)
		time.Sleep(5 * time.Second)
	}

	return nil, fmt.Errorf("falha ao conectar ao RabbitMQ após %d tentativas: %w", maxRetries, err)
}

func (p *AMQPPublisher) connect() error {
	conn, err := amqp.Dial(p.amqpURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to declare an exchange: %w", err)
	}

	p.conn = conn
	p.channel = ch
	return nil
}

func (p *AMQPPublisher) Publish(ctx context.Context, cameraID string, payload []byte) error {
	routingKey := p.routingKeyPrefix + cameraID
	err := p.channel.Publish(
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/octet-stream",
			Body:        payload,
			Timestamp:   time.Now(),
		})
	if err != nil {
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	return nil
}

func (p *AMQPPublisher) Close() error {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// GetChannel returns the underlying AMQP channel.
func (p *AMQPPublisher) GetChannel() *amqp.Channel {
	return p.channel
}

// ExtractVhostFromURL extracts the vhost from an AMQP URL.
// Example: amqp://user:pass@localhost:5672/myvhost -> "myvhost"
// If no vhost is specified, returns "/" (default vhost).
func ExtractVhostFromURL(amqpURL string) (string, error) {
	parsedURL, err := url.Parse(amqpURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse AMQP URL: %w", err)
	}

	vhost := strings.TrimPrefix(parsedURL.Path, "/")
	if vhost == "" {
		vhost = "/"
	}

	return vhost, nil
}
