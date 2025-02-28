package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Event represents an interface for writing event data to an io.Writer.
// Implementations of this interface must provide the Write method,
// which takes an io.Writer and returns an error if the write operation fails.
type Event interface {
	Write(r io.Writer) error
}

// TextEvent represents a simple event structure with a name and associated data.
// It is used to encapsulate information for events in the SSE (Server-Sent Events) protocol.
type TextEvent struct {
	Name string
	Data string
}

// Write formats the TextEvent as a string and writes it to the provided io.Writer.
// It outputs the event name and data in the SSE format, followed by two newlines.
// Returns an error if the write operation fails.
func (e *TextEvent) Write(w io.Writer) error {
	// Write event header.
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(e.Name)
	b.WriteString("\n")

	// Split the data into lines.
	lines := strings.Split(e.Data, "\n")
	// Build the SSE response.
	for _, line := range lines {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("data:\n\n")

	// Write the complete output.
	_, err := io.WriteString(w, b.String())
	return err
}

// JsonEvent represents an event with a name and associated data.
// It can be used to structure events in a JSON format in the SSE (Server-Sent Events) protocol.
type JsonEvent struct {
	Name string
	Data any
}

// Write serializes the JsonEvent to the provided io.Writer in the SSE format.
// It writes the event name and the JSON-encoded data, followed by a double newline
// to indicate the end of the event. If an error occurs during marshaling or writing,
// it returns the error.
func (e *JsonEvent) Write(w io.Writer) error {
	buf, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, string(buf))

	return err
}
