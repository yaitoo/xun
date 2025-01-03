package xun

import (
	"net/http"
)

type ResponseWriter interface {
	http.ResponseWriter

	Close()
}

type responseWriter struct {
	http.ResponseWriter
}

func (w *responseWriter) Close() {

}
