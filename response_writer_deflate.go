package xun

import (
	"compress/flate"
)

// deflateResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using the deflate algorithm.
type deflateResponseWriter struct {
	*stdResponseWriter
	w *flate.Writer
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
func (rw *deflateResponseWriter) Write(p []byte) (int, error) {
	n, err := rw.w.Write(p)
	rw.bodySentBytes += n
	return n, err
}

// Close closes the underlying writer, flushing any buffered data to the client.
// It is important to call this method to ensure all data is properly sent.
func (rw *deflateResponseWriter) Close() {
	rw.w.Close()
}

// Flush writes any buffered data to the underlying writer and ensures that
// the response is sent to the client. It locks the response writer to
// prevent concurrent writes, flushes the compressed data, and then
// flushes the standard response writer.
func (rw *deflateResponseWriter) Flush() {
	rw.w.Flush()
	rw.stdResponseWriter.Flush()
}
