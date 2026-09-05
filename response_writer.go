package xun

import (
	"net/http"
)

// ResponseWriter is an interface that extends the standard http.ResponseWriter
// interface with BodyBytesSent, StatusCode, Close, http.Flusher, and
// http.Hijacker. The Flusher and Hijacker additions let HandleFunc bodies
// access http.Flusher and http.Hijacker through the wrapped writer without
// dropping down to app.Mux().Handle(...).
//
// Out-of-tree implementations must provide Flush and Hijack. A stub Hijack
// that returns errors.New("not supported") is acceptable when the wrapped
// writer cannot be hijacked (e.g. test recorders).
type ResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	http.Hijacker

	BodyBytesSent() int
	StatusCode() int
	Close()
}
