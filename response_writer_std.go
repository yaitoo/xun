package xun

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

// stdResponseWriter is a wrapper around http.ResponseWriter to implement the ResponseWriter interface.
type stdResponseWriter struct {
	http.ResponseWriter
	bodySentBytes int
	statusCode    int
	// hijacked is set true after Hijack transfers ownership of the underlying
	// connection to the caller. Compression wrappers (gzip, deflate) consult
	// this flag to avoid writing their trailer bytes onto a connection the
	// caller now owns.
	hijacked bool
}

// Close implements the ResponseWriter interface Close method.
// It is a no-op for the standard response writer.
func (*stdResponseWriter) Close() {
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
// allowing the response writer to flush the response immediately. When the wrapped
// ResponseWriter does not implement http.Flusher (e.g. some test recorders), Flush is a
// no-op.
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

// Hijack implements the http.Hijacker interface. It returns the underlying
// net.Conn and *bufio.ReadWriter from the wrapped http.ResponseWriter when
// the wrapped writer itself implements http.Hijacker (e.g. *http.response
// from a real net/http server). When the wrapped writer does not implement
// http.Hijacker (e.g. httptest.ResponseRecorder), Hijack returns an error.
//
// Use Hijack from a HandleFunc to integrate with handlers that require
// raw access to the underlying connection, such as WebSocket upgrades.
//
// Calling Hijack transfers ownership of the connection to the caller.
// After a successful Hijack, compression wrappers (gzip, deflate) skip
// their trailer flush on Close so the caller-owned bytes are not corrupted.
// The hijacked flag is set only on success: if Hijack returns an error,
// the wrapper keeps working through the encoder as before.
//
// Body writes before Hijack are at the caller's risk: gzip/deflate buffers
// response bytes internally; if a body has been written and Hijack is then
// called, the gzipResponseWriter / deflateResponseWriter Hijack flushes the
// encoder before transferring ownership. Callers should generally write
// only headers (and WriteHeader for the upgrade status) before Hijack.
func (rw *stdResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("xun: underlying ResponseWriter does not implement Hijacker")
	}
	conn, buf, err := h.Hijack()
	if err != nil {
		return nil, nil, err
	}
	rw.hijacked = true
	return conn, buf, nil
}
