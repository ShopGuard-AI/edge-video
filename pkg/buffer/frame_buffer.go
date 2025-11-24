package buffer

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Frame representa um frame aguardando processamento.
type Frame struct {
	CameraID  string
	Data      []byte
	Timestamp time.Time
	Release   func()
}

type FrameBuffer struct {
	buffer        chan Frame
	capacity      int
	droppedFrames int64
	totalFrames   int64
}

func NewFrameBuffer(capacity int) *FrameBuffer {
	return &FrameBuffer{
		buffer:   make(chan Frame, capacity),
		capacity: capacity,
	}
}

func (fb *FrameBuffer) Push(frame Frame) error {
	atomic.AddInt64(&fb.totalFrames, 1)

	select {
	case fb.buffer <- frame:
		return nil
	default:
		// Buffer cheio: descarta o frame mais antigo para dar lugar ao novo
		select {
		case dropped := <-fb.buffer:
			if dropped.Release != nil {
				dropped.Release()
			}
		default:
		}
		fb.buffer <- frame
		atomic.AddInt64(&fb.droppedFrames, 1)
		return fmt.Errorf("buffer cheio: frame substituído")
	}
}

func (fb *FrameBuffer) Pop() (Frame, bool) {
	select {
	case frame := <-fb.buffer:
		return frame, true
	default:
		return Frame{}, false
	}
}

func (fb *FrameBuffer) PopBlocking(ctx context.Context) (Frame, bool) {
	select {
	case <-ctx.Done():
		return Frame{}, false
	case frame, ok := <-fb.buffer:
		return frame, ok
	}
}

func (fb *FrameBuffer) Size() int {
	return len(fb.buffer)
}

func (fb *FrameBuffer) Capacity() int {
	return fb.capacity
}

func (fb *FrameBuffer) Stats() BufferStats {
	dropped := atomic.LoadInt64(&fb.droppedFrames)
	total := atomic.LoadInt64(&fb.totalFrames)

	dropRate := float64(0)
	if total > 0 {
		dropRate = float64(dropped) / float64(total) * 100
	}

	return BufferStats{
		Size:          fb.Size(),
		Capacity:      fb.capacity,
		DroppedFrames: dropped,
		TotalFrames:   total,
		DropRate:      dropRate,
	}
}

func (fb *FrameBuffer) Close() {
	close(fb.buffer)
}

type BufferStats struct {
	Size          int
	Capacity      int
	DroppedFrames int64
	TotalFrames   int64
	DropRate      float64
}

func (bs BufferStats) String() string {
	return fmt.Sprintf("Buffer: %d/%d, Total: %d, Dropped: %d (%.2f%%)",
		bs.Size, bs.Capacity, bs.TotalFrames, bs.DroppedFrames, bs.DropRate)
}
