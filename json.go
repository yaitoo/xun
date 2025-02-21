package xun

import (
	"encoding/json"
	"io"
)

var Json JsonEncoding = &stdJsonEncoding{}

// JsonEncoding is the interface that defines the methods that the standard
// library encoding/json package provides.
type JsonEncoding interface {
	NewEncoder(writer io.Writer) Encoder
	NewDecoder(reader io.Reader) Decoder
}

type Decoder interface {
	Decode(obj interface{}) error
}

type Encoder interface {
	Encode(val interface{}) error
}

type stdJsonEncoding struct {
}

func (*stdJsonEncoding) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

func (*stdJsonEncoding) NewDecoder(reader io.Reader) Decoder {
	return json.NewDecoder(reader)
}
