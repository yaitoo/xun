package sse

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrServerClosed  = errors.New("sse: server closed")
	ErrClientTimeout = errors.New("sse: client timeout")
)

// Client represents a connection to a streaming service.
// It holds the client's ID, a Streamer instance for managing the stream,
// a context for cancellation and timeout, and a channel for signaling closure.
type Client struct {
	mu       sync.Mutex
	ID       string
	connID   int
	sm       Streamer
	ctx      context.Context
	cancel   context.CancelCauseFunc
	lastSeen time.Time
}

// Send sends an event to the client by writing the event name and data to the response writer.
// It marshals the event data into JSON format and flushes the output to ensure the data is sent immediately.
// This method is part of the Client struct and is intended for use in server-sent events (SSE) communication.
func (c *Client) Send(evt Event) error {
	if c.ctx.Err() != nil {
		return NewError(c.ID, context.Cause(c.ctx))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	err := evt.Write(c.sm)
	if err != nil {
		return NewError(c.ID, err)
	}

	c.lastSeen = time.Now()
	c.sm.Flush()

	return nil
}

func (c *Client) Context() context.Context {
	return c.ctx
}

// Wait blocks until the context is done or the client is closed.
// It listens for either the cancellation of the context or a signal
// to close the client, allowing for graceful shutdown.
func (c *Client) Wait(ctx context.Context) error {

	select {
	case <-c.ctx.Done():
		return context.Cause(c.ctx)
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// Close gracefully shuts down the Client by sending a signal to the close channel.
// This method should be called to ensure that any ongoing operations are properly terminated.
func (c *Client) Close() {
	c.cancel(ErrServerClosed)
}
