package xun

import (
	"compress/flate"
	"net/http"
)

// deflateResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using the deflate algorithm.
type deflateResponseWriter struct {
	w *flate.Writer
	http.ResponseWriter
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
func (w *deflateResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

// Close closes the underlying writer, flushing any buffered data to the client.
// It is important to call this method to ensure all data is properly sent.
func (w *deflateResponseWriter) Close() {
	w.w.Close()
}
