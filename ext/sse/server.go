// Package sse provides a server implementation for Server-Sent Events (SSE).
// SSE is a technology enabling a client to receive automatic updates from a server via HTTP connection.
package sse

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/yaitoo/async"
)

// Server represents a structure that manages connected clients
// in a concurrent environment. It uses a read-write mutex to
// ensure safe access to the clients map, which holds the
// active Client instances identified by their unique keys.
type Server struct {
	mu      sync.RWMutex
	clients map[string]*Client
	conns   map[string]int
	ctx     context.Context
	cancel  context.CancelCauseFunc

	keepalive time.Duration
}

// New creates and returns a new instance of the Server struct.
func New(opts ...Option) *Server {
	ctx, cf := context.WithCancelCause(context.Background())
	s := &Server{
		clients: make(map[string]*Client),
		conns:   make(map[string]int),
		ctx:     ctx,
		cancel:  cf,
	}

	for _, opt := range opts {
		opt(s)
	}

	go s.keepAlive()
	return s
}

func (s *Server) keepAlive() {
	if s.keepalive <= 0 {
		return
	}

	ticker := time.NewTicker(s.keepalive)
	defer ticker.Stop()

	var now, timeout, ping time.Time
	var deadClients []*Client
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			deadClients = make([]*Client, 0, len(s.clients))
			now = time.Now()
			timeout = now.Add(-s.keepalive)
			ping = now.Add(-s.keepalive / 2)
			for _, c := range s.clients {
				if c.lastSeen.Before(timeout) {
					if c.lastSeen.Before(ping) {
						c.Send(&PingEvent{}) //nolint: errcheck
					}

					continue
				}

				deadClients = append(deadClients, c)

			}
			s.mu.RUnlock()

			if len(deadClients) > 0 {
				s.mu.Lock()
				for _, c := range deadClients {
					c.cancel(ErrClientTimeout)
					delete(s.clients, c.ID)
					delete(s.conns, c.ID)
				}
				s.mu.Unlock()
			}
		}
	}
}

// Join adds a new client to the server.
// It establishes a connection with the specified Streamer and sets the appropriate headers
// for Server-Sent Events (SSE).
func (s *Server) Join(clientID string, rw http.ResponseWriter) (*Client, int, error) {
	sm, err := NewStreamer(rw)
	if err != nil {
		return nil, 0, err
	}

	s.mu.Lock()
	c, ok := s.clients[clientID]

	if !ok {
		c = &Client{
			ID:     clientID,
			connID: 1,
		}

		s.clients[clientID] = c
		s.conns[clientID] = 1
		c.ctx, c.cancel = context.WithCancelCause(s.ctx)

	} else {
		c.connID = s.conns[clientID] + 1
		s.conns[clientID] = c.connID
	}

	c.sm = sm
	s.mu.Unlock()

	sm.Header().Set("Content-Type", "text/event-stream")
	sm.Header().Set("Cache-Control", "no-cache")
	sm.Header().Set("Connection", "keep-alive")
	sm.Flush()

	return c, c.connID, nil
}

// Leave removes a client from the server's client list by its ID.
// This method is safe for concurrent use, as it locks the server
// before modifying the clients map and ensures that the lock is
// released afterward.
func (s *Server) Leave(clientID string, connID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.clients[clientID]
	if !ok {
		return true
	}

	if c.connID != connID {
		return false
	}

	delete(s.clients, clientID)
	delete(s.conns, clientID)

	c.cancel(ErrServerClosed)

	return true
}

// Get retrieves the Client associated with the given id from the Server.
// It uses a read lock to ensure thread-safe access to the clients map.
// Returns nil if no Client is found for the specified id.
func (s *Server) Get(id string) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[id]
}

// Broadcast sends the specified event to all connected clients.
// It acquires a read lock to ensure thread-safe access to the clients slice,
// and spawns a goroutine for each client to handle the sending of the event.
func (s *Server) Broadcast(ctx context.Context, event Event) ([]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := async.NewA()

	for _, c := range s.clients {

		tasks.Add(func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return NewError(c.ID, context.Cause(ctx))
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
	s.cancel(ErrServerClosed)

	s.clients = make(map[string]*Client)
	s.conns = make(map[string]int)
}
