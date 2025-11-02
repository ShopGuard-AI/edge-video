package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/streadway/amqp"
)

type AMQPPublisher struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	exchange         string
	routingKeyPrefix string
}

func NewAMQPPublisher(amqpURL, exchange, routingKeyPrefix string) (*AMQPPublisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare an exchange: %w", err)
	}

	return &AMQPPublisher{
		conn:             conn,
		channel:          ch,
		exchange:         exchange,
		routingKeyPrefix: routingKeyPrefix,
	}, nil
}

func (p *AMQPPublisher) Publish(ctx context.Context, cameraID string, payload []byte) error {
	routingKey := p.routingKeyPrefix + "." + cameraID
	err := p.channel.Publish(
		p.exchange,   // exchange
		routingKey,   // routing key
		false,        // mandatory
		false,        // immediate
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
