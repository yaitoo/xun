package sse

import "net/http"

type Streamer interface {
	http.ResponseWriter
	http.Flusher
}
