package xun

import (
	"strings"
)

// host, path, pattern
func splitFile(s string) (host string, path string, pattern string) {
	defer func() {
		if pattern[len(pattern)-1] == '/' {
			pattern = pattern + "{$}"
		}
	}()

	if len(s) == 0 {
		pattern = "GET /"
		return
	}

	i := strings.IndexByte(s, '@')

	// xxx
	if i < 0 { // no host
		path = "/" + s        // /xxx
		pattern = "GET /" + s // GET /xxx
		return
	}

	// @abc.com/xxx
	e := strings.IndexByte(s, '/')
	// has host
	host = s[i+1 : e]          // abc.com
	path = "/" + s[e+1:]       // /xxx
	pattern = "GET " + s[i+1:] // GET abc.com/xxx
	return
}
