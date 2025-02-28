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
	n, err := rw.w.Write(p)
	rw.bodySentBytes += n
	return n, err
}

// Close closes the gzipResponseWriter, ensuring that the underlying writer is also closed.
func (rw *gzipResponseWriter) Close() {
	rw.w.Close()
}

// Flush writes any buffered data to the underlying writer and ensures that
// the gzip response writer is properly synchronized. It locks the mutex
// to prevent concurrent access, flushes the gzip writer, and then flushes
// the standard response writer.
func (rw *gzipResponseWriter) Flush() {
	rw.w.Flush()
	rw.stdResponseWriter.Flush()
}
