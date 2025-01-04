package xun

import "net/http"

// stdResponseWriter is a wrapper around http.ResponseWriter to implement the ResponseWriter interface.
type stdResponseWriter struct {
	http.ResponseWriter
}

// Close implements the ResponseWriter interface Close method.
// It is a no-op for the standard response writer.
func (w *stdResponseWriter) Close() {
}
