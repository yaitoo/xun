package htmx

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
func splitFile(s string) (string, string, string) {
	if len(s) == 0 {
		return "", "", ""
	}

	i := strings.IndexByte(s, '@')

	if i < 0 { //no host
		return "", "/" + s, "/" + s
	}

	e := strings.IndexByte(s, '/')
	//has host
	return s[i+1 : e], "/" + s[e+1:], s[i+1:]
}
