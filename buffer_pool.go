package xun

import (
	"bytes"
)

// BufferPool is a pool of *bytes.Buffer for reuse to reduce memory alloc.
type BufferPool struct {
	c chan *bytes.Buffer
}

// NewBufferPool returns a new BufferPool with the given size.
//
// The size determines how many buffers can be stored in the pool. If the
// pool is full and a new buffer is requested, a new buffer will be created.
func NewBufferPool(size int) (bp *BufferPool) {
	return &BufferPool{
		c: make(chan *bytes.Buffer, size),
	}
}

// Get retrieves a buffer from the pool or creates a new one if the pool is empty.
//
// If a buffer is available in the pool, it is returned for reuse, reducing memory
// allocations. If the pool is empty, a new buffer is created and returned.
func (bp *BufferPool) Get() (b *bytes.Buffer) {
	select {
	case b = <-bp.c:
	// reuse existing buffer
	default:
		// create new buffer
		b = bytes.NewBuffer([]byte{})
	}
	return
}

// Put returns a buffer to the pool for reuse or discards if the pool is full.
//
// This function resets the buffer to clear any existing data before
// returning it to the pool. If the pool is already full, the buffer is discarded.
func (bp *BufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	select {
	case bp.c <- b:
	default: // Discard the buffer if the pool is full.
	}
}
