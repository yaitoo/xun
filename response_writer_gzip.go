package xun

import (
	"compress/gzip"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	*stdResponseWriter
	w *gzip.Writer
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
func (rw *gzipResponseWriter) Write(p []byte) (int, error) {
	return rw.w.Write(p)
}

// Close closes the gzipResponseWriter, ensuring that the underlying writer is also closed.
func (rw *gzipResponseWriter) Close() {
	rw.w.Close()
}
