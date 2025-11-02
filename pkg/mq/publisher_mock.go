package mq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockPublisher struct {
	PublishFunc func(ctx context.Context, cameraID string, payload []byte) error
	CloseFunc   func() error
}

func (m *MockPublisher) Publish(ctx context.Context, cameraID string, payload []byte) error {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, cameraID, payload)
	}
	return nil
}

func (m *MockPublisher) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestMockPublisher(t *testing.T) {
	var published bool
	mock := &MockPublisher{
		PublishFunc: func(ctx context.Context, cameraID string, payload []byte) error {
			published = true
			assert.Equal(t, "cam1", cameraID)
			assert.Equal(t, []byte("test"), payload)
			return nil
		},
	}

	err := mock.Publish(context.Background(), "cam1", []byte("test"))
	assert.NoError(t, err)
	assert.True(t, published)
}