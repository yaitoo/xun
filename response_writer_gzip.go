package xun

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
)

// gzipResponseWriter is a custom http.ResponseWriter that wraps the standard
// ResponseWriter and compresses the response using gzip.
type gzipResponseWriter struct {
	*stdResponseWriter
	w *gzip.Writer
	// closed is set after the first Close returns the encoder to the
	// pool. A second Close is a no-op so the same *gzip.Writer is
	// never Put into gzipWriterPool twice; two concurrent Gets of
	// the same encoder pointer would corrupt shared deflate state
	// across requests. Pre-pool this was harmless because
	// gzip.Writer.Close is idempotent; pooling turned double-Close
	// into a correctness hazard.
	closed bool
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
//
// After flushing the gzip trailer, the *gzip.Writer is returned to
// gzipWriterPool so the inner *flate.Writer's deflate state (64 KiB
// sliding window + fast-encoder history buffer) is recycled across
// requests. Reset(io.Discard) before Put drops the residual reference to
// the previous ResponseWriter so the pooled encoder does not pin the
// per-request conn alive.
//
// If Hijack has transferred ownership of the connection to the caller,
// Close is a no-op: the gzip trailer must NOT be written onto the
// caller-owned stream, and the encoder must NOT be returned to the pool,
// because reusing an encoder whose destination was the caller-owned conn
// would write pooled-state bytes back into a stream the framework no longer
// owns. The hijacked guard runs before any pool access.
//
// Close is idempotent: a second call after a successful Close is a
// no-op so the same encoder is never Put twice. Idempotency is
// required because handlers may explicitly call Close in addition to
// the framework's defer.
func (rw *gzipResponseWriter) Close() {
	if rw.closed || rw.hijacked {
		return
	}
	rw.closed = true
	rw.w.Close() // nolint: errcheck
	rw.w.Reset(io.Discard)
	gzipWriterPool.Put(rw.w)
	rw.w = nil
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
