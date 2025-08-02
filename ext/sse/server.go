// Package sse provides a server implementation for Server-Sent Events (SSE).
// SSE is a technology enabling a client to receive automatic updates from a server via HTTP connection.
package sse

import (
	"context"
	"net/http"
	"sync"

	"github.com/yaitoo/async"
)

// Server represents a structure that manages connected clients
// in a concurrent environment. It uses a read-write mutex to
// ensure safe access to the clients map, which holds the
// active Client instances identified by their unique keys.
type Server struct {
	mu    sync.RWMutex
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

	s.mu.Lock()
	_, ok := s.conns[id]

	if ok {
		s.mu.Unlock()
		return nil, ErrClientJoined
	}

	c := &Conn{
		ID: id,
	}

	s.conns[id] = c

	c.Connect(ctx, sm)
	s.mu.Unlock()

	sm.Header().Set("Content-Type", "text/event-stream")
	sm.Header().Set("Cache-Control", "no-cache")
	sm.Header().Set("Connection", "keep-alive")
	sm.Flush()

	return c, nil
}

// MustJoin creates or replaces a connection with the given id, associates it with the provided
// http.ResponseWriter, and initializes a new SSE (Server-Sent Events) stream. If a connection
// with the same id already exists, it is closed and replaced. The function sets appropriate
// headers for SSE, flushes the initial response, and returns the new connection. If the
// streamer cannot be created, an error is returned.
func (s *Server) MustJoin(ctx context.Context, id string, rw http.ResponseWriter) (*Conn, error) {
	sm, err := NewStreamer(rw)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	c, ok := s.conns[id]

	if ok {
		c.Close()
	} else {
		c = &Conn{
			ID: id,
		}
		s.conns[id] = c
	}

	c.Connect(ctx, sm)
	s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.conns, id)
}

// Get retrieves the Client associated with the given id from the Server.
// It uses a read lock to ensure thread-safe access to the clients map.
// Returns nil if no Client is found for the specified id.
func (s *Server) Get(id string) *Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conns[id]
}

// Broadcast sends the specified event to all connected clients.
// It acquires a read lock to ensure thread-safe access to the clients slice,
// and spawns a goroutine for each client to handle the sending of the event.
func (s *Server) Broadcast(ctx context.Context, event Event) ([]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.Close()
	}

	s.conns = make(map[string]*Conn)
}
