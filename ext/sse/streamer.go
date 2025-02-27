package sse

import (
	"errors"
	"net/http"
)

var ErrNotStreamer = errors.New("sse: not streamer")

type Streamer interface {
	http.ResponseWriter
	http.Flusher
}

type stdStreamer struct {
	http.ResponseWriter
	http.Flusher
}

// NewStreamer creates a new Streamer instance from the provided http.ResponseWriter.
// It returns an error if the ResponseWriter is nil or does not implement the http.Flusher interface.
// This function is intended for use in handling server-sent events (SSE).
func NewStreamer(rw http.ResponseWriter) (Streamer, error) {
	if rw == nil {
		return nil, ErrNotStreamer
	}

	flusher, ok := rw.(http.Flusher)
	if !ok {
		return nil, ErrNotStreamer
	}
	return &stdStreamer{rw, flusher}, nil
}
