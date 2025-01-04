package xun

import (
	"net/http"
)

// ResponseWriter is an interface that extends the standard http.ResponseWriter
// interface with an additional Close method. It is used to write HTTP responses
// and perform any necessary cleanup or finalization when the response is complete.
type ResponseWriter interface {
	http.ResponseWriter

	Close()
}

// responseWriter is a wrapper around http.ResponseWriter that allows for
// additional functionality to be added to the standard ResponseWriter.
type responseWriter struct {
	http.ResponseWriter
}

func (w *responseWriter) Close() {
}