package xun

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type MimeType struct {
	Type    string
	SubType string
}

func NewMimeType(t string) MimeType {
	items := strings.Split(t, "/")

	mt := MimeType{
		Type:    items[0],
		SubType: "*",
	}

	if len(items) > 1 {
		mt.SubType = items[1]
	}

	return mt

}

func (m *MimeType) Match(accept MimeType) bool {
	if m.Type != accept.Type && (m.Type != "*" && accept.Type != "*") {
		return false
	}

	if m.SubType == accept.SubType || (m.SubType == "*" || accept.SubType == "*") {
		return true
	}

	return false
}

func (m *MimeType) String() string {
	return m.Type + "/" + m.SubType
}

func GetMimeType(file string, buf []byte) (MimeType, string) {
	mt := mime.TypeByExtension(filepath.Ext(file))
	if mt == "" {
		mt = http.DetectContentType(buf)
	}

	// text/plain; charset=utf-8
	i := strings.Index(mt, ";")
	if i == -1 {
		return NewMimeType(mt), "; charset=utf-8"
	}
	return NewMimeType(mt[:i]), mt[i:]
}
