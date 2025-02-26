package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client represents a WebSocket client that handles HTTP responses and supports
// flushing data to the client. It contains a response writer, a flusher for
// sending data immediately, and a channel for managing the client's lifecycle.
type Client struct {
	rw      http.ResponseWriter
	flusher http.Flusher
}

// Connect establishes a connection for the Client using the provided Streamer.
// It assigns the Streamer to the Client's rw field and ensures that it implements
// the http.Flusher interface for flushing data.
func (c *Client) Connect(rw Streamer) {
	c.rw = rw
	c.flusher = rw.(http.Flusher)
}

// Wait blocks until the context is done or an event occurs.
// It continuously checks the context's Done channel and performs
// actions in the default case. This function is intended to be
// used in a long-running process where it can be interrupted
// by the provided context.
func (c *Client) Wait(ctx context.Context) {
	<-ctx.Done()
}

// Send sends an event to the client by writing the event name and data to the response writer.
// It marshals the event data into JSON format and flushes the output to ensure the data is sent immediately.
// This method is part of the Client struct and is intended for use in server-sent events (SSE) communication.
func (c *Client) Send(event Event) {
	buf, _ := json.Marshal(event.Data)
	fmt.Fprintf(c.rw, "event: %s\n", event.Name)
	fmt.Fprintf(c.rw, "data: %s\n\n", string(buf))
	c.flusher.Flush()
}
