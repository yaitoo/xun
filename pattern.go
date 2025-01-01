package xun

import (
	"strings"
)

func splitPattern(s string) (string, string, string) {
	if len(s) == 0 {
		return "", "", ""
	}

	method, rest, found := s, "", false
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		method, rest, found = s[:i], strings.TrimLeft(s[i+1:], " \t"), true
	}
	if !found {
		rest = method
		method = ""
	}

	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return method, "", rest
	}
	return method, rest[:i], rest[i+1:]
}

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
	if i < 0 { //no host
		path = "/" + s        // /xxx
		pattern = "GET /" + s // GET /xxx
		return
	}

	// @abc.com/xxx
	e := strings.IndexByte(s, '/')
	//has host
	host = s[i+1 : e]          // abc.com
	path = "/" + s[e+1:]       // /xxx
	pattern = "GET " + s[i+1:] // GET abc.com/xxx
	return
}
