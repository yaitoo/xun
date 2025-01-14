package xun

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func GetMimeType(file string, buf []byte) (string, string) {
	mt := mime.TypeByExtension(filepath.Ext(file))
	if mt == "" {
		mt = http.DetectContentType(buf)
	}

	// text/plain; charset=utf-8
	i := strings.Index(mt, ";")
	if i == -1 {
		return mt, "; charset=utf-8"
	}
	return mt[:i], mt[i:]
}
