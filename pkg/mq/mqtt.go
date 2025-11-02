package mq

import (
	"context"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTPublisher struct {
	client      mqtt.Client
	topicPrefix string
}

func NewMQTTPublisher(broker, topicPrefix string) (*MQTTPublisher, error) {
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}
	log.Println("Conectado ao broker MQTT")
	return &MQTTPublisher{client: client, topicPrefix: topicPrefix}, nil
}

func (p *MQTTPublisher) Publish(ctx context.Context, cameraID string, payload []byte) error {
	topic := p.topicPrefix + cameraID
	token := p.client.Publish(topic, 1, false, payload)
	sent := token.Wait()
	if !sent {
		return fmt.Errorf("o cliente MQTT não conseguiu enviar a mensagem")
	}
	if token.Error() != nil {
		return fmt.Errorf("falha ao publicar no MQTT: %w", token.Error())
	}
	return nil
}

func (p *MQTTPublisher) Close() error {
	p.client.Disconnect(250)
	return nil
}