package htmx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPattern(t *testing.T) {
	tests := []struct {
		pattern string
		method  string
		host    string
		path    string
	}{
		{"", "", "", ""},

		{"/", "", "", ""},
		{"/abc", "", "", "abc"},

		{"GET /abc", "GET", "", "abc"},
		{"GET /abc/", "GET", "", "abc/"},

		{"GET abc.com/", "GET", "abc.com", ""},
		{"GET abc.com/abc", "GET", "abc.com", "abc"},
		{"GET abc.com/abc/", "GET", "abc.com", "abc/"},
	}

	for _, test := range tests {
		method, host, path := splitPattern(test.pattern)

		require.Equal(t, test.method, method, test.pattern)
		require.Equal(t, test.host, host, test.pattern)
		require.Equal(t, test.path, path, test.pattern)

	}
}
