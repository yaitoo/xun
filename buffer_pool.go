package htmx

import (
	"bytes"
)

type BufferPool struct {
	c chan *bytes.Buffer
}

func NewBufferPool(size int) (bp *BufferPool) {
	return &BufferPool{
		c: make(chan *bytes.Buffer, size),
	}
}

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

func (bp *BufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	select {
	case bp.c <- b:
	default: // Discard the buffer if the pool is full.
	}
}
