package sse

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClientClosed = errors.New("sse: client closed")
)

// Conn represents a connection to a streaming service.
// It holds the client's ID, a Streamer instance for managing the stream,
// a context for cancellation and timeout, and a channel for signaling closure.
type Conn struct {
	sync.Mutex
	ID     string
	s      Streamer
	ctx    context.Context
	cancel context.CancelFunc
}

// Connect establishes a connection for the Client using the provided Streamer.
// It assigns the Streamer to the Client's rw field and ensures that it implements
// the http.Flusher interface for flushing data.
func (c *Conn) Connect(ctx context.Context, s Streamer) {
	c.s = s
	c.ctx, c.cancel = context.WithCancel(ctx)
}

// Send sends an event to the client by writing the event name and data to the response writer.
// It marshals the event data into JSON format and flushes the output to ensure the data is sent immediately.
// This method is part of the Client struct and is intended for use in server-sent events (SSE) communication.
func (c *Conn) Send(evt Event) error {
	if c.ctx.Err() != nil {
		return NewError(c.ID, ErrClientClosed)
	}

	c.Lock()
	defer c.Unlock()

	err := evt.Write(c.s)
	if err != nil {
		return NewError(c.ID, err)
	}

	c.s.Flush()

	return nil
}

// Wait blocks until the context is done or the client is closed.
// It listens for either the cancellation of the context or a signal
// to close the client, allowing for graceful shutdown.
func (c *Conn) Wait() {
	<-c.ctx.Done()
}

// Close gracefully shuts down the Client by sending a signal to the close channel.
// This method should be called to ensure that any ongoing operations are properly terminated.
func (c *Conn) Close() {
	c.cancel()
}
