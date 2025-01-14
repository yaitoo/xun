package xun

import (
	"mime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMime(t *testing.T) {
	mime.AddExtensionType(".xun1", "xun/mime; charset=utf-16") // nolint: errcheck
	mime.AddExtensionType(".xun2", "xun/mime")                 // nolint: errcheck

	t.Run("mime_charset", func(t *testing.T) {
		mime, charset := GetMimeType("test.xun1", []byte{})
		require.Equal(t, "xun/mime", mime)
		require.Equal(t, "; charset=utf-16", charset)

	})

	t.Run("mime_no_charset", func(t *testing.T) {
		mime, charset := GetMimeType("test.xun2", []byte{})
		require.Equal(t, "xun/mime", mime)
		require.Equal(t, "; charset=utf-8", charset)

	})

	t.Run("http_mime", func(t *testing.T) {
		mime, charset := GetMimeType("test.xun3", []byte(`<?xml version="1.0" encoding="UTF-8"?>`))
		require.Equal(t, "text/xml", mime)
		require.Equal(t, "; charset=utf-8", charset)
	})
}
