package metadata

import (
	"encoding/json"
	"time"

	"github.com/streadway/amqp"
)

// Publisher handles publishing frame metadata to RabbitMQ.
type Publisher struct {
	channel    *amqp.Channel
	exchange   string
	routingKey string
	enabled    bool
}

// NewPublisher creates a new metadata Publisher.
func NewPublisher(ch *amqp.Channel, exchange, routingKey string, enabled bool) *Publisher {
	// Se estiver habilitado e o canal existir, declara o exchange
	if enabled && ch != nil {
		err := ch.ExchangeDeclare(
			exchange,
			"topic",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			// Log o erro mas não falha a criação do publisher
			// Isso permite que a aplicação continue mesmo se o exchange já existir
			// ou houver problemas temporários
		}
	}

	return &Publisher{
		channel:    ch,
		exchange:   exchange,
		routingKey: routingKey,
		enabled:    enabled,
	}
}

// Enabled returns true if the metadata publisher is enabled.
func (p *Publisher) Enabled() bool {
	return p.enabled
}

// Metadata represents the structure of the metadata message.
type Metadata struct {
	CameraID   string    `json:"camera_id"`
	Timestamp  time.Time `json:"timestamp"`
	RedisKey   string    `json:"redis_key"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Encoding   string    `json:"encoding"`
	SizeBytes  int       `json:"size_bytes"`
}

// PublishMetadata sends a JSON message with frame metadata to RabbitMQ.
func (p *Publisher) PublishMetadata(cameraID string, timestamp time.Time, redisKey string, width, height, size int, encoding string) error {
	if !p.enabled {
		return nil
	}

	metadata := Metadata{
		CameraID:   cameraID,
		Timestamp:  timestamp,
		RedisKey:   redisKey,
		Width:      width,
		Height:     height,
		Encoding:   encoding,
		SizeBytes:  size,
	}

	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return p.channel.Publish(
		p.exchange,
		p.routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
