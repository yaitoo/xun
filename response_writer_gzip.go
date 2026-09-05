package xun

import (
	"bufio"
	"compress/gzip"
	"net"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	*stdResponseWriter
	w *gzip.Writer
}

// Write writes the data to the underlying gzip writer.
// It implements the io.Writer interface.
//
// After a successful Hijack, Write is a no-op so the gzip encoder does not
// emit compressed bytes onto the caller-owned stream.
func (rw *gzipResponseWriter) Write(p []byte) (int, error) {
	if rw.hijacked {
		return len(p), nil
	}
	n, err := rw.w.Write(p)
	rw.bodySentBytes += n
	return n, err
}

// Close closes the gzipResponseWriter, ensuring that the underlying writer is also closed.
// If Hijack has transferred ownership of the connection to the caller, Close
// is a no-op so the gzip trailer is not written onto the caller-owned stream.
func (rw *gzipResponseWriter) Close() {
	if rw.hijacked {
		return
	}
	rw.w.Close() // nolint: errcheck
}

// Flush writes any buffered data to the underlying writer and then flushes
// the standard response writer. After Hijack transfers ownership of the
// underlying connection, Flush is a no-op so compressed bytes are not
// written onto the caller-owned stream.
func (rw *gzipResponseWriter) Flush() {
	if rw.hijacked {
		return
	}
	rw.w.Flush() // nolint: errcheck
	rw.stdResponseWriter.Flush()
}

// Hijack implements http.Hijacker. Before transferring ownership to the
// caller, the gzip writer is flushed so any bytes already written through
// the encoder are not lost. Callers should generally write only the upgrade
// status and headers before Hijack — body writes before Hijack are at the
// caller's risk because the compressed stream and the raw post-hijack
// bytes share the same conn.
func (rw *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rw.w != nil {
		_ = rw.w.Flush()
	}
	return rw.stdResponseWriter.Hijack()
}
