package xun

import "net/http"

// Compressor is an interface that defines methods for handling HTTP response compression.
// Implementations of this interface should provide the following methods:
//
// AcceptEncoding returns the encoding type that the compressor supports.
//
// WriteTo takes an http.ResponseWriter and returns a wrapped http.ResponseWriter
// that compresses the response data.
type Compressor interface {
	AcceptEncoding() string
	New(rw http.ResponseWriter) ResponseWriter
}
