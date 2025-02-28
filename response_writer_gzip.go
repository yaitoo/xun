package xun

import (
	"compress/gzip"
	"net/http"
	"sync"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	mu sync.Mutex
	*stdResponseWriter
	w *gzip.Writer
}

func (rw *gzipResponseWriter) Header() http.Header {
	return rw.stdResponseWriter.ResponseWriter.Header()
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
func (rw *gzipResponseWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	n, err := rw.w.Write(p)
	rw.bodySentBytes += n
	return n, err
}

// Close closes the gzipResponseWriter, ensuring that the underlying writer is also closed.
func (rw *gzipResponseWriter) Close() {
	rw.w.Close()
}

func (rw *gzipResponseWriter) Flush() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.w.Flush()
	rw.stdResponseWriter.Flush()
}
