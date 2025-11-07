package mq

import "context"

type Publisher interface {
	Publish(ctx context.Context, cameraID string, payload []byte) error
	Close() error
}
