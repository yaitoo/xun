package xun

import (
	"compress/gzip"
	"net/http"
)

// GzipCompressor is a struct that provides methods for compressing and decompressing data using the Gzip algorithm.
type GzipCompressor struct {
}

// AcceptEncoding returns the encoding type that the GzipCompressor supports.
// In this case, it returns "gzip".
func (c *GzipCompressor) AcceptEncoding() string {
	return "gzip"
}

// New creates a new gzipResponseWriter that wraps the provided http.ResponseWriter.
// It sets the "Content-Encoding" header to "gzip" and returns the wrapped writer.
func (c *GzipCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "gzip")

	return &gzipResponseWriter{
		w:              gzip.NewWriter(rw),
		ResponseWriter: rw,
	}

}
