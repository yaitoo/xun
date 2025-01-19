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
		require.Equal(t, "xun/mime", mime.String())
		require.True(t, mime.Match(MimeType{Type: "xun", SubType: "mime"}))
		require.Equal(t, "; charset=utf-16", charset)

	})

	t.Run("mime_no_charset", func(t *testing.T) {
		mime, charset := GetMimeType("test.xun2", []byte{})
		require.Equal(t, "xun/mime", mime.String())
		require.True(t, mime.Match(MimeType{Type: "xun", SubType: "mime"}))
		require.Equal(t, "; charset=utf-8", charset)
	})

	t.Run("http_mime", func(t *testing.T) {
		mime, charset := GetMimeType("test.xun3", []byte(`<?xml version="1.0" encoding="UTF-8"?>`))
		require.Equal(t, "text/xml", mime.String())
		require.True(t, mime.Match(MimeType{Type: "text", SubType: "xml"}))
		require.Equal(t, "; charset=utf-8", charset)
	})

	tests := []struct {
		name string
		m1   MimeType
		m2   MimeType
		want bool
	}{
		{
			name: "full_match",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "text", SubType: "html"},
			want: true,
		},
		{
			name: "wildcard_sub_match",
			m1:   MimeType{Type: "text", SubType: "*"},
			m2:   MimeType{Type: "text", SubType: "html"},
			want: true,
		},
		{
			name: "wildcard_sub_match_2",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "text", SubType: "*"},
			want: true,
		},
		{
			name: "wildcard_type_match",
			m1:   MimeType{Type: "*", SubType: "html"},
			m2:   MimeType{Type: "text", SubType: "html"},
			want: true,
		},
		{
			name: "wildcard_type_match_2",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "*", SubType: "html"},
			want: true,
		},
		{
			name: "wildcard_match_2",
			m1:   MimeType{Type: "*", SubType: "*"},
			m2:   MimeType{Type: "text", SubType: "html"},
			want: true,
		},
		{
			name: "wildcard_match_2",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "*", SubType: "*"},
			want: true,
		},
		{
			name: "no_match",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "text", SubType: "json"},
			want: false,
		},
		{
			name: "no_match",
			m1:   MimeType{Type: "text", SubType: "html"},
			m2:   MimeType{Type: "application", SubType: "json"},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched := test.m1.Match(test.m2)
			require.Equal(t, test.want, matched)
		})
	}

}
