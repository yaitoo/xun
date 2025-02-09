package xun

import "net/http"

// stdResponseWriter is a wrapper around http.ResponseWriter to implement the ResponseWriter interface.
type stdResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// Close implements the ResponseWriter interface Close method.
// It is a no-op for the standard response writer.
func (*stdResponseWriter) Close() {
}

func (rw *stdResponseWriter) WriteHeader(statusCode int) {
	if rw.statusCode == 0 {
		rw.statusCode = statusCode
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *stdResponseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func NewResponseWriter(rw http.ResponseWriter) ResponseWriter {
	return &stdResponseWriter{ResponseWriter: rw}
}
