package xun

import (
	"compress/flate"
	"net/http"
)

// DeflateCompressor is a struct that provides functionality for compressing data using the DEFLATE algorithm.
type DeflateCompressor struct {
}

// AcceptEncoding returns the encoding type that the DeflateCompressor supports.
// In this case, it returns the string "deflate".
func (c *DeflateCompressor) AcceptEncoding() string {
	return "deflate"
}

// New creates a new deflateResponseWriter that wraps the provided http.ResponseWriter.
// It sets the "Content-Encoding" header to "deflate" and initializes a flate.Writer
// with the default compression level.
func (c *DeflateCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "deflate")
	w, _ := flate.NewWriter(rw, flate.DefaultCompression) //nolint: errcheck because flate.DefaultCompression is a valid compression level

	return &deflateResponseWriter{
		w:              w,
		ResponseWriter: rw,
	}
}
