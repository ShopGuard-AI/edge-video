package mq

import "context"

// Publisher provides an abstraction for sending messages from captures.
type Publisher interface {
	Publish(ctx context.Context, cameraID string, payload []byte) error
	Close() error
}