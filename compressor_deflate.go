package xun

import (
	"compress/flate"
	"io"
	"net/http"
	"sync"
)

// DeflateCompressor is a struct that provides functionality for compressing data using the DEFLATE algorithm.
type DeflateCompressor struct {
}

// deflateWriterPool recycles *flate.Writer across requests. A fresh
// flate.Writer at DefaultCompression carries a 64 KiB sliding window plus
// an up-to-320 KiB fast-encoder history buffer (~384 KiB total).
// Allocating that state per request dominated the GC profile of
// compressed apps. The pool is goroutine-safe: each pooled encoder is
// bound to a single request between Get and Put, and Reset on Get / Put
// drops any residual reference to the previous ResponseWriter.
var deflateWriterPool = sync.Pool{
	New: func() any {
		// DefaultCompression is a valid compression level; flate.NewWriter
		// only errors when the level is out of range.
		w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression) //nolint: errcheck
		return w
	},
}

// AcceptEncoding returns the encoding type that the DeflateCompressor supports.
// In this case, it returns the string "deflate".
func (c *DeflateCompressor) AcceptEncoding() string {
	return "deflate"
}

// New creates a new deflateResponseWriter that wraps the provided http.ResponseWriter.
// It sets the "Content-Encoding" header to "deflate" and binds a flate.Writer
// (acquired from deflateWriterPool) to the underlying writer.
//
// The *flate.Writer is reused via Reset on every call. Reset rebinds the
// destination io.Writer and clears the deflate state, so the 64 KiB
// sliding window and fast-encoder history buffer are recycled across
// requests instead of being reallocated on every compressed response.
func (c *DeflateCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "deflate")
	w := deflateWriterPool.Get().(*flate.Writer)
	w.Reset(rw)

	return &deflateResponseWriter{
		w: w,
		stdResponseWriter: &stdResponseWriter{
			ResponseWriter: rw,
		},
	}
}