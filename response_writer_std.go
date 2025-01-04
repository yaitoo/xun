package xun

import "net/http"

// stdResponseWriter is a wrapper around http.ResponseWriter that allows for
// additional functionality to be added to the standard ResponseWriter.
type stdResponseWriter struct {
	http.ResponseWriter
}

func (w *stdResponseWriter) Close() {
}
