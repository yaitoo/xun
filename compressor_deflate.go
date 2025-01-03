package xun

import (
	"compress/flate"
	"net/http"
)

type DeflateCompressor struct {
}

func (c *DeflateCompressor) AcceptEncoding() string {
	return "deflate"
}

func (c *DeflateCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "deflate")
	w, _ := flate.NewWriter(rw, flate.DefaultCompression)
	return &deflateResponseWriter{
		w:              w,
		ResponseWriter: rw,
	}

}
