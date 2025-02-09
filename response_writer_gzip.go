package xun

import (
	"compress/gzip"
	"net/http"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	http.ResponseWriter
	w          *gzip.Writer
	statusCode int
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

func (rw *gzipResponseWriter) WriteHeader(statusCode int) {
	if rw.statusCode == 0 {
		rw.statusCode = statusCode
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *gzipResponseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}
