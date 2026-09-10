package xun

import (
	"compress/gzip"
	"io"
	"net/http"
	"sync"
)

// GzipCompressor is a struct that provides methods for compressing and decompressing data using the Gzip algorithm.
type GzipCompressor struct {
}

// gzipWriterPool recycles *gzip.Writer across requests. A fresh gzip.Writer
// carries a 4 KiB bufio.Writer plus a ~256 KiB deflate state at
// DefaultCompression; allocating it per request dominated the GC profile of
// compressed apps. The pool is goroutine-safe: each pooled encoder is bound
// to a single request between Get and Put, and Reset on Get / Put drops any
// residual reference to the previous ResponseWriter.
var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// AcceptEncoding returns the encoding type that the GzipCompressor supports.
// In this case, it returns "gzip".
func (c *GzipCompressor) AcceptEncoding() string {
	return "gzip"
}

// New creates a new gzipResponseWriter that wraps the provided http.ResponseWriter.
// It sets the "Content-Encoding" header to "gzip" and returns the wrapped writer.
//
// The *gzip.Writer is acquired from gzipWriterPool and rebound to rw via
// Reset, so the internal deflate state and bufio buffer are recycled across
// requests instead of being reallocated on every compressed response.
func (c *GzipCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "gzip")

	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(rw)

	return &gzipResponseWriter{
		w: gz,
		stdResponseWriter: &stdResponseWriter{
			ResponseWriter: rw,
		},
	}
}