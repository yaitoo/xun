package xun

import (
	"encoding/json"
	"io"
)

var Json JsonEncoding

func init() {
	Json = &stdJsonEncoding{}
}

// JsonEncoding is the interface that defines the methods that the standard
// library encoding/json package provides.
type JsonEncoding interface {
	Marshal(v interface{}) ([]byte, error)
	MarshalIndent(v interface{}, prefix, indent string) ([]byte, error)
	UnmarshalFromString(str string, v interface{}) error
	Unmarshal(data []byte, v interface{}) error
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

func (e *stdJsonEncoding) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (e *stdJsonEncoding) MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

func (e *stdJsonEncoding) UnmarshalFromString(str string, v interface{}) error {
	return json.Unmarshal([]byte(str), v)
}

func (e *stdJsonEncoding) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (e *stdJsonEncoding) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

func (e *stdJsonEncoding) NewDecoder(reader io.Reader) Decoder {
	return json.NewDecoder(reader)
}
