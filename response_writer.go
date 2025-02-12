package xun

import (
	"net/http"
)

// ResponseWriter is an interface that extends the standard http.ResponseWriter
// interface with an additional Close method. It is used to write HTTP responses
// and perform any necessary cleanup or finalization when the response is complete.
type ResponseWriter interface {
	http.ResponseWriter

	BodyBytesSent() int
	StatusCode() int
	Close()
}
