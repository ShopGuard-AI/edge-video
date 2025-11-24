package mq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

type AMQPPublisher struct {
	mu               sync.RWMutex
	conn             *amqp.Connection
	channel          *amqp.Channel
	notifyClose      chan *amqp.Error
	exchange         string
	routingKeyPrefix string
	amqpURL          string
	closed           bool
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

	if err := ch.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to declare an exchange: %w", err)
	}

	notify := make(chan *amqp.Error, 1)
	ch.NotifyClose(notify)

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		ch.Close()
		conn.Close()
		return errors.New("publisher closed")
	}
	p.conn = conn
	p.channel = ch
	p.notifyClose = notify
	p.mu.Unlock()

	go p.handleConnectionClose(notify)

	return nil
}

func (p *AMQPPublisher) Publish(ctx context.Context, cameraID string, payload []byte) error {
	routingKey := p.routingKeyPrefix + cameraID
	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		ch := p.getChannel()
		if ch == nil {
			if recErr := p.reconnect(); recErr != nil {
				lastErr = recErr
				break
			}
			ch = p.getChannel()
			if ch == nil {
				lastErr = fmt.Errorf("amqp channel unavailable after reconnect")
				break
			}
		}

		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}

		err := ch.Publish(
			p.exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/octet-stream",
				Body:        payload,
				Timestamp:   time.Now(),
			},
		)
		if err == nil {
			return nil
		}

		lastErr = err

		if !p.shouldReconnect(err) {
			break
		}

		if recErr := p.reconnect(); recErr != nil {
			lastErr = fmt.Errorf("%w; reconnect failed: %v", err, recErr)
			break
		}
	}

	return fmt.Errorf("failed to publish a message: %w", lastErr)
}

func (p *AMQPPublisher) Close() error {
	p.mu.Lock()
	p.closed = true
	ch := p.channel
	conn := p.conn
	p.channel = nil
	p.conn = nil
	p.notifyClose = nil
	p.mu.Unlock()

	var err error
	if ch != nil {
		err = ch.Close()
	}
	if conn != nil {
		if cerr := conn.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// GetChannel returns the underlying AMQP channel.
func (p *AMQPPublisher) GetChannel() *amqp.Channel {
	return p.channel
}

// ExtractVhostFromURL extracts the vhost from an AMQP URL.
// Example: amqp://user:pass@localhost:5672/myvhost -> "myvhost"
// If no vhost is specified, returns "/" (default vhost).
func ExtractVhostFromURL(amqpURL string) (string, error) {
	if !strings.HasPrefix(amqpURL, "amqp://") && !strings.HasPrefix(amqpURL, "amqps://") {
		return "", fmt.Errorf("invalid amqp url: %s", amqpURL)
	}

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

func (p *AMQPPublisher) getChannel() *amqp.Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.channel
}

func (p *AMQPPublisher) reconnect() error {
	if p.isClosed() {
		return errors.New("publisher closed")
	}

	p.closeCurrent()

	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second}
	var err error
	for attempt, wait := range backoff {
		if err = p.connect(); err == nil {
			log.Printf("Reconectado ao RabbitMQ na tentativa %d", attempt+1)
			return nil
		}
		log.Printf("Falha ao reconectar ao RabbitMQ (%d/%d): %v", attempt+1, len(backoff), err)
		time.Sleep(wait)
	}
	return err
}

func (p *AMQPPublisher) closeCurrent() {
	p.mu.Lock()
	ch := p.channel
	conn := p.conn
	p.channel = nil
	p.conn = nil
	p.notifyClose = nil
	p.mu.Unlock()

	if ch != nil {
		_ = ch.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (p *AMQPPublisher) handleConnectionClose(notify <-chan *amqp.Error) {
	err, ok := <-notify
	if !ok || p.isClosed() {
		return
	}

	if err != nil {
		log.Printf("Canal RabbitMQ fechado: %v", err)
	} else {
		log.Printf("Canal RabbitMQ fechado sem erro")
	}

	if recErr := p.reconnect(); recErr != nil {
		log.Printf("Falha ao reconectar ao RabbitMQ após fechamento do canal: %v", recErr)
	}
}

func (p *AMQPPublisher) isClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

func (p *AMQPPublisher) shouldReconnect(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, amqp.ErrClosed) {
		return true
	}

	msg := err.Error()
	if strings.Contains(msg, "channel/connection is not open") {
		return true
	}

	if strings.Contains(msg, "connection closed") {
		return true
	}

	if strings.Contains(msg, "EOF") {
		return true
	}

	return false
}
