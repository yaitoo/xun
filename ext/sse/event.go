package sse

// Event represents a server-sent event with a name and associated data.
// It can be used to transmit information from the server to the client in real-time.
type Event struct {
	Name string
	Data any
}
