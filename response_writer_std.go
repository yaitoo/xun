package xun

import "net/http"

// stdResponseWriter is a wrapper around http.ResponseWriter to implement the ResponseWriter interface.
type stdResponseWriter struct {
	http.ResponseWriter
	bodySentBytes int
	statusCode    int
}

// Close implements the ResponseWriter interface Close method.
// It is a no-op for the standard response writer.
func (*stdResponseWriter) Close() {
}

func (rw *stdResponseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

// WriteHeader sends an HTTP response header with the specified status code.
// It ensures that the header is only written once by checking if the statusCode
// has already been set. If the statusCode is zero, it updates the statusCode
// and calls the underlying ResponseWriter's WriteHeader method to send the header.
func (rw *stdResponseWriter) WriteHeader(statusCode int) {
	if rw.statusCode == 0 {
		rw.statusCode = statusCode
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// StatusCode returns the HTTP status code of the response writer.
// If the status code has not been set, it defaults to http.StatusOK.
func (rw *stdResponseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

// BodyBytesSent returns the number of bytes sent in the response body.
// It is a method of the stdResponseWriter type and provides access
// to the internal byte count for monitoring or logging purposes.
func (rw *stdResponseWriter) BodyBytesSent() int {
	return rw.bodySentBytes
}

// Write writes the data to the underlying ResponseWriter and tracks the number of bytes sent.
// It returns the number of bytes written and any error encountered during the write operation.
func (rw *stdResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)

	rw.bodySentBytes = rw.bodySentBytes + n

	return n, err
}

// Flush sends any buffered data to the client. It implements the http.Flusher interface,
// allowing the response writer to flush the response immediately.
func (rw *stdResponseWriter) Flush() {
	f, ok := rw.ResponseWriter.(http.Flusher)
	if ok {
		f.Flush()
	}
}

// NewResponseWriter creates a new instance of ResponseWriter that wraps the provided http.ResponseWriter.
// It returns a pointer to a stdResponseWriter, which implements the ResponseWriter interface.
func NewResponseWriter(rw http.ResponseWriter) ResponseWriter {
	return &stdResponseWriter{ResponseWriter: rw}
}
