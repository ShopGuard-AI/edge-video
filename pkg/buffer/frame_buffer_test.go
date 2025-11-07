package buffer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFrameBuffer(t *testing.T) {
	buffer := NewFrameBuffer(10)
	
	assert.NotNil(t, buffer)
	assert.Equal(t, 10, buffer.Capacity())
	assert.Equal(t, 0, buffer.Size())
}

func TestFrameBufferPush(t *testing.T) {
	buffer := NewFrameBuffer(5)
	
	frame := Frame{
		CameraID:  "cam1",
		Data:      []byte("test data"),
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]interface{}{"key": "value"},
	}
	
	err := buffer.Push(frame)
	assert.NoError(t, err)
	assert.Equal(t, 1, buffer.Size())
}

func TestFrameBufferPushFull(t *testing.T) {
	buffer := NewFrameBuffer(2)
	
	frame1 := Frame{CameraID: "cam1", Data: []byte("data1"), Timestamp: 1}
	frame2 := Frame{CameraID: "cam2", Data: []byte("data2"), Timestamp: 2}
	frame3 := Frame{CameraID: "cam3", Data: []byte("data3"), Timestamp: 3}
	
	err := buffer.Push(frame1)
	assert.NoError(t, err)
	
	err = buffer.Push(frame2)
	assert.NoError(t, err)
	
	err = buffer.Push(frame3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer cheio")
	
	stats := buffer.Stats()
	assert.Equal(t, int64(1), stats.DroppedFrames)
	assert.Equal(t, int64(3), stats.TotalFrames)
}

func TestFrameBufferPop(t *testing.T) {
	buffer := NewFrameBuffer(5)
	
	frame := Frame{
		CameraID:  "cam1",
		Data:      []byte("test"),
		Timestamp: 123456,
	}
	
	_ = buffer.Push(frame)
	
	popped, ok := buffer.Pop()
	assert.True(t, ok)
	assert.Equal(t, "cam1", popped.CameraID)
	assert.Equal(t, []byte("test"), popped.Data)
	assert.Equal(t, int64(123456), popped.Timestamp)
}

func TestFrameBufferPopEmpty(t *testing.T) {
	buffer := NewFrameBuffer(5)
	
	_, ok := buffer.Pop()
	assert.False(t, ok)
}

func TestFrameBufferStats(t *testing.T) {
	buffer := NewFrameBuffer(3)
	
	for i := 0; i < 5; i++ {
		frame := Frame{
			CameraID:  "cam1",
			Data:      []byte("data"),
			Timestamp: int64(i),
		}
		_ = buffer.Push(frame)
	}
	
	stats := buffer.Stats()
	
	assert.Equal(t, int64(5), stats.TotalFrames)
	assert.Equal(t, int64(2), stats.DroppedFrames)
	assert.Equal(t, 3, stats.Size)
	assert.Equal(t, 3, stats.Capacity)
	
	dropRate := (float64(2) / float64(5)) * 100
	assert.InDelta(t, dropRate, stats.DropRate, 0.01)
}

func TestFrameBufferClose(t *testing.T) {
	buffer := NewFrameBuffer(5)
	
	frame := Frame{CameraID: "cam1", Data: []byte("test")}
	_ = buffer.Push(frame)
	
	buffer.Close()
	
	_, ok := buffer.PopBlocking()
	assert.True(t, ok)
	
	_, ok = buffer.PopBlocking()
	assert.False(t, ok)
}

func TestFrameBufferConcurrent(t *testing.T) {
	buffer := NewFrameBuffer(100)
	
	done := make(chan bool)
	
	go func() {
		for i := 0; i < 50; i++ {
			frame := Frame{
				CameraID:  "cam1",
				Data:      []byte("data"),
				Timestamp: int64(i),
			}
			_ = buffer.Push(frame)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()
	
	go func() {
		for i := 0; i < 50; i++ {
			buffer.Pop()
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()
	
	<-done
	<-done
	
	stats := buffer.Stats()
	assert.Equal(t, int64(50), stats.TotalFrames)
}

func BenchmarkFrameBufferPush(b *testing.B) {
	buffer := NewFrameBuffer(10000)
	
	frame := Frame{
		CameraID:  "cam1",
		Data:      make([]byte, 1024),
		Timestamp: time.Now().Unix(),
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = buffer.Push(frame)
	}
}

func BenchmarkFrameBufferPop(b *testing.B) {
	buffer := NewFrameBuffer(10000)
	
	for i := 0; i < 10000; i++ {
		frame := Frame{
			CameraID:  "cam1",
			Data:      []byte("data"),
			Timestamp: int64(i),
		}
		_ = buffer.Push(frame)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		buffer.Pop()
	}
}
