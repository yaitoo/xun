// Package sse provides a server implementation for Server-Sent Events (SSE).
// SSE is a technology enabling a client to receive automatic updates from a server via HTTP connection.
package sse

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/yaitoo/async"
)

var ErrClientJoined = errors.New("sse: client already joined")

// Server represents a structure that manages connected clients
// in a concurrent environment. It uses a read-write mutex to
// ensure safe access to the clients map, which holds the
// active Client instances identified by their unique keys.
type Server struct {
	sync.RWMutex
	conns map[string]*Conn
}

// New creates and returns a new instance of the Server struct.
func New() *Server {
	return &Server{
		conns: make(map[string]*Conn),
	}
}

// Join adds a new client to the server.
// It establishes a connection with the specified Streamer and sets the appropriate headers
// for Server-Sent Events (SSE).
func (s *Server) Join(ctx context.Context, id string, rw http.ResponseWriter) (*Conn, error) {
	sm, err := NewStreamer(rw)
	if err != nil {
		return nil, err
	}

	s.RLock()
	_, ok := s.conns[id]
	s.RUnlock()

	if !ok {
		return nil, ErrClientClosed
	}

	c := &Conn{
		ID: id,
	}

	s.Unlock()
	s.conns[id] = c
	s.Unlock()

	c.Connect(ctx, sm)

	sm.Header().Set("Content-Type", "text/event-stream")
	sm.Header().Set("Cache-Control", "no-cache")
	sm.Header().Set("Connection", "keep-alive")
	sm.Flush()

	return c, nil
}

// Leave removes a client from the server's client list by its ID.
// This method is safe for concurrent use, as it locks the server
// before modifying the clients map and ensures that the lock is
// released afterward.
func (s *Server) Leave(id string) {
	s.Lock()
	defer s.Unlock()

	delete(s.conns, id)
}

// Get retrieves the Client associated with the given id from the Server.
// It uses a read lock to ensure thread-safe access to the clients map.
// Returns nil if no Client is found for the specified id.
func (s *Server) Get(id string) *Conn {
	s.RLock()
	defer s.RUnlock()
	return s.conns[id]
}

// Broadcast sends the specified event to all connected clients.
// It acquires a read lock to ensure thread-safe access to the clients slice,
// and spawns a goroutine for each client to handle the sending of the event.
func (s *Server) Broadcast(ctx context.Context, event Event) ([]error, error) {
	s.RLock()
	defer s.RUnlock()

	tasks := async.NewA()

	for _, c := range s.conns {

		tasks.Add(func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return NewError(c.ID, err)
			}
			if err := c.Send(event); err != nil {
				return NewError(c.ID, err)
			}
			return nil
		})
	}

	return tasks.Wait(ctx)
}

// Shutdown gracefully closes all active client connections and cleans up the client list.
// It locks the server to ensure thread safety during the shutdown process.
func (s *Server) Shutdown() {
	s.Lock()
	defer s.Unlock()
	for _, c := range s.conns {
		c.Close()
	}

	s.conns = make(map[string]*Conn)
}
