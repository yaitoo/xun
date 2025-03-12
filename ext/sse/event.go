package sse

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"strconv"
	"strings"
)

var (
	bufPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	sbPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
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
	ID    string
	Name  string
	Retry int
	Data  string
}

// Write formats the TextEvent as a string and writes it to the provided io.Writer.
// It outputs the event name and data in the SSE format, followed by two newlines.
// Returns an error if the write operation fails.
func (e *TextEvent) Write(w io.Writer) error {
	sb := sbPool.Get().(*strings.Builder)
	defer sbPool.Put(sb)
	sb.Reset()

	if e.ID != "" {
		sb.WriteString("id: ")
		sb.WriteString(e.ID)
		sb.WriteString("\n")
	}

	if e.Retry > 0 {
		sb.WriteString("retry: ")
		sb.WriteString(strconv.Itoa(e.Retry))
		sb.WriteString("\n")
	}

	if e.Name != "" {
		sb.WriteString("event: ")
		sb.WriteString(e.Name)
		sb.WriteString("\n")
	}

	// Split the data into lines.
	lines := strings.Split(e.Data, "\n")
	// Build the SSE response.
	for _, line := range lines {
		sb.WriteString("data: ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Write the complete output.
	_, err := io.WriteString(w, sb.String())
	return err
}

// JsonEvent represents an event with a name and associated data.
// It can be used to structure events in a JSON format in the SSE (Server-Sent Events) protocol.
type JsonEvent struct {
	ID    string
	Name  string
	Retry int
	Data  any
}

// Write serializes the JsonEvent to the provided io.Writer in the SSE format.
// It writes the event name and the JSON-encoded data, followed by a double newline
// to indicate the end of the event. If an error occurs during marshaling or writing,
// it returns the error.
func (e *JsonEvent) Write(w io.Writer) error {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()

	err := json.NewEncoder(buf).Encode(e.Data)

	if err != nil {
		return err
	}

	sb := sbPool.Get().(*strings.Builder)
	defer sbPool.Put(sb)
	sb.Reset()

	if e.ID != "" {
		sb.WriteString("id: ")
		sb.WriteString(e.ID)
		sb.WriteString("\n")
	}

	if e.Retry > 0 {
		sb.WriteString("retry: ")
		sb.WriteString(strconv.Itoa(e.Retry))
		sb.WriteString("\n")
	}

	if e.Name != "" {
		sb.WriteString("event: ")
		sb.WriteString(e.Name)
		sb.WriteString("\n")
	}

	// Split the data into lines.
	lines := strings.Split(buf.String(), "\n")
	// Build the SSE response.
	for _, line := range lines {
		if line != "" {
			sb.WriteString("data: ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Write the complete output.
	_, err = io.WriteString(w, sb.String())
	return err
}

type PingEvent struct {
}

func (evt *PingEvent) Write(w io.Writer) error {
	_, err := io.WriteString(w, ": ping\n\n")
	return err
}
