package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrClientClosed = errors.New("sse: client closed")

// Client represents a WebSocket client that handles HTTP responses and supports
// flushing data to the client. It contains a response writer, a flusher for
// sending data immediately, and a channel for managing the client's lifecycle.
type Client struct {
	s Streamer

	ctx context.Context
}

// Connect establishes a connection for the Client using the provided Streamer.
// It assigns the Streamer to the Client's rw field and ensures that it implements
// the http.Flusher interface for flushing data.
func (c *Client) Connect(ctx context.Context, s Streamer) {
	c.s = s
	c.ctx = ctx
}

// Send sends an event to the client by writing the event name and data to the response writer.
// It marshals the event data into JSON format and flushes the output to ensure the data is sent immediately.
// This method is part of the Client struct and is intended for use in server-sent events (SSE) communication.
func (c *Client) Send(event Event) error {
	select {
	case <-c.ctx.Done():
		return ErrClientClosed
	default:
		buf, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.s, "event: %s\ndata: %s\n\n", event.Name, string(buf))
		if err != nil {
			return err
		}

		c.s.Flush()
	}

	return nil
}
