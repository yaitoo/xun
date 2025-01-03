package xun

import (
	"compress/gzip"
	"net/http"
)

type gzipResponseWriter struct {
	w *gzip.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *gzipResponseWriter) Close() {
	w.w.Close()
}
