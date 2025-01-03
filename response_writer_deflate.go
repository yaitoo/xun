package xun

import (
	"compress/flate"
	"net/http"
)

type deflateResponseWriter struct {
	w *flate.Writer
	http.ResponseWriter
}

func (w *deflateResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *deflateResponseWriter) Close() {
	w.w.Close()
}
