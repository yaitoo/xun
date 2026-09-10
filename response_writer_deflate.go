package xun

import (
	"bufio"
	"compress/flate"
	"io"
	"net"
)

// deflateResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using the deflate algorithm.
type deflateResponseWriter struct {
	*stdResponseWriter
	w *flate.Writer
	// closed is set after the first Close returns the encoder to the
	// pool. A second Close is a no-op so the same *flate.Writer is
	// never Put into deflateWriterPool twice; two concurrent Gets of
	// the same encoder pointer would corrupt deflate state across
	// requests. Pre-pool this was harmless because deflate.Writer.Close
	// is idempotent; pooling turned double-Close into a correctness
	// hazard.
	closed bool
}

// Write writes the data to the underlying deflate writer.
// It implements the io.Writer interface.
//
// After a successful Hijack, Write is a no-op so the deflate encoder does not
// emit compressed bytes onto the caller-owned stream. After Close, Write is
// also a no-op so a stray write through a wrapper whose encoder has already
// been returned to the pool does not panic on the nil rw.w nor pull a
// fresh encoder out of the pool into a half-closed wrapper.
func (rw *deflateResponseWriter) Write(p []byte) (int, error) {
	if rw.hijacked || rw.closed {
		return len(p), nil
	}
	n, err := rw.w.Write(p)
	rw.bodySentBytes += n
	return n, err
}

// Close closes the underlying writer, flushing any buffered data to the client.
// It is important to call this method to ensure all data is properly sent.
//
// After flushing, the *flate.Writer is returned to deflateWriterPool so the
// internal deflate state is recycled across requests. Reset(io.Discard)
// before Put drops the residual reference to the previous ResponseWriter so
// the pooled encoder does not pin the per-request conn alive.
//
// If Hijack has transferred ownership of the connection to the caller,
// Close is a no-op: the deflate trailer must NOT be written onto the
// caller-owned stream, and the encoder must NOT be returned to the pool,
// because reusing an encoder whose destination was the caller-owned conn
// would write pooled-state bytes back into a stream the framework no longer
// owns. The hijacked guard runs before any pool access.
//
// Close is idempotent: a second call after a successful Close is a
// no-op so the same encoder is never Put twice. Idempotency is
// required because handlers may explicitly call Close in addition to
// the framework's defer.
func (rw *deflateResponseWriter) Close() {
	if rw.closed || rw.hijacked {
		return
	}
	rw.closed = true
	rw.w.Close() // nolint: errcheck
	rw.w.Reset(io.Discard)
	deflateWriterPool.Put(rw.w)
	rw.w = nil
}

// Flush writes any buffered data to the underlying writer and then flushes
// the standard response writer. After Hijack transfers ownership of the
// underlying connection, Flush is a no-op so compressed bytes are not
// written onto the caller-owned stream. After Close, Flush is also a
// no-op so a stray flush does not panic on the nil rw.w.
func (rw *deflateResponseWriter) Flush() {
	if rw.hijacked || rw.closed {
		return
	}
	rw.w.Flush() // nolint: errcheck
	rw.stdResponseWriter.Flush()
}

// Hijack implements http.Hijacker. Before transferring ownership to the
// caller, the deflate writer is flushed so any bytes already written through
// the encoder are not lost. Callers should generally write only the upgrade
// status and headers before Hijack — body writes before Hijack are at the
// caller's risk because the compressed stream and the raw post-hijack
// bytes share the same conn.
func (rw *deflateResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rw.w != nil {
		_ = rw.w.Flush()
	}
	return rw.stdResponseWriter.Hijack()
}
