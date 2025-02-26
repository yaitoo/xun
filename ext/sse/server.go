// Package sse provides a server implementation for Server-Sent Events (SSE).
// SSE is a technology enabling a client to receive automatic updates from a server via HTTP connection.
package sse

import (
	"sync"
)

// Server represents a structure that manages connected clients
// in a concurrent environment. It uses a read-write mutex to
// ensure safe access to the clients map, which holds the
// active Client instances identified by their unique keys.
type Server struct {
	sync.RWMutex
	clients map[string]*Client
}

// New creates and returns a new instance of the Server struct.
func New() *Server {
	return &Server{
		clients: make(map[string]*Client),
	}
}

// Join adds a new client to the server or retrieves an existing one based on the provided ID.
// It establishes a connection with the specified Streamer and sets the appropriate headers
// for Server-Sent Events (SSE). If a client with the given ID already exists, it reuses that client.
func (s *Server) Join(id string, sm Streamer) *Client {
	s.Lock()
	defer s.Unlock()
	c, ok := s.clients[id]

	if !ok {
		c = &Client{}
		s.clients[id] = c
	}

	c.Connect(sm)

	sm.Header().Set("Content-Type", "text/event-stream")
	sm.Header().Set("Cache-Control", "no-cache")
	sm.Header().Set("Connection", "keep-alive")

	return c
}

// Leave removes a client from the server's client list by its ID.
// This method is safe for concurrent use, as it locks the server
// before modifying the clients map and ensures that the lock is
// released afterward.
func (s *Server) Leave(id string) {
	s.Lock()
	defer s.Unlock()

	delete(s.clients, id)
}

// Get retrieves the Client associated with the given id from the Server.
// It uses a read lock to ensure thread-safe access to the clients map.
// Returns nil if no Client is found for the specified id.
func (s *Server) Get(id string) *Client {
	s.RLock()
	defer s.RUnlock()
	return s.clients[id]
}

// Broadcast sends the specified event to all connected clients.
// It acquires a read lock to ensure thread-safe access to the clients slice,
// and spawns a goroutine for each client to handle the sending of the event.
func (s *Server) Broadcast(event Event) {
	s.RLock()
	defer s.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(s.clients))
	for _, c := range s.clients {
		go func() {
			defer wg.Done()
			c.Send(event)
		}()
	}

	wg.Wait()
}
