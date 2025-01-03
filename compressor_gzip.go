package xun

import (
	"compress/gzip"
	"net/http"
)

type GzipCompressor struct {
}

func (c *GzipCompressor) AcceptEncoding() string {
	return "gzip"
}

func (c *GzipCompressor) New(rw http.ResponseWriter) ResponseWriter {
	rw.Header().Set("Content-Encoding", "gzip")

	return &gzipResponseWriter{
		w:              gzip.NewWriter(rw),
		ResponseWriter: rw,
	}

}
