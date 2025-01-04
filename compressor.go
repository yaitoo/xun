package xun

import "net/http"

// Compressor is an interface that defines methods for handling HTTP response compression.
// Implementations of this interface should provide the specific encoding type they support
// and a method to create a new ResponseWriter that applies the compression.
//
// AcceptEncoding returns the encoding type that the compressor supports.
//
// New takes an http.ResponseWriter and returns a ResponseWriter that applies the compression.
type Compressor interface {
	AcceptEncoding() string
	New(rw http.ResponseWriter) ResponseWriter
}
