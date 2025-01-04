package xun

import (
	"compress/gzip"
	"net/http"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	w *gzip.Writer
	http.ResponseWriter
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

// Close closes the gzipResponseWriter, ensuring that the underlying writer is also closed.
func (w *gzipResponseWriter) Close() {
	w.w.Close()
}
