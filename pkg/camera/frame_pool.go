package camera

import "sync"

const maxFramePoolSize = 2000 * 1024 * 1024 // 2MB

var framePool = sync.Pool{
	New: func() any {
		return make([]byte, 0, maxFramePoolSize)
	},
}

func getFrameBuffer(size int) []byte {
	if size <= 0 {
		return nil
	}

	if size > maxFramePoolSize {
		return make([]byte, size)
	}

	buf := framePool.Get().([]byte)
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	return buf[:size]
}

func releaseFrameBuffer(buf []byte) {
	if buf == nil {
		return
	}

	if cap(buf) > maxFramePoolSize {
		return
	}

	framePool.Put(buf[:0])
}
