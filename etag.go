package xun

import (
	// skipcq: GSC-G401, GSC-G501, GO-S1023
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"net/textproto"
	"strings"
)

// ComputeETag returns the ETag header value for the given reader content.
//
// The value is computed by taking the crc32 of the content and encoding it
// as a hexadecimal string.
func ComputeETag(r io.Reader) string {
	h := crc32.NewIEEE()
	return ComputeETagWith(r, h)
}

// ComputeETagWith returns the ETag header value for the given reader content
// using the provided hash function.
func ComputeETagWith(r io.Reader, h hash.Hash) string {
	if _, err := io.Copy(h, r); err != nil {
		return ""
	}

	return `"` + hex.EncodeToString(h.Sum(nil)) + `"`
}

func WriteIfNoneMatch(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" {
		if checkIfNoneMatch(w, r) {
			writeNotModified(w)
			return true
		}
	}

	return false
}

func checkIfNoneMatch(w http.ResponseWriter, r *http.Request) bool {
	inm := r.Header.Get("If-None-Match")
	if inm == "" {
		return false
	}
	buf := inm
	for {
		buf = textproto.TrimString(buf)
		if len(buf) == 0 {
			break
		}
		if buf[0] == ',' {
			buf = buf[1:]
			continue
		}
		if buf[0] == '*' {
			return true
		}
		etag, remain := scanETag(buf)
		if etag == "" {
			break
		}
		if etagWeakMatch(etag, w.Header().Get("Etag")) {
			return true
		}
		buf = remain
	}
	return false
}

// scanETag determines if a syntactically valid ETag is present at s. If so,
// the ETag and remaining text after consuming ETag is returned. Otherwise,
// it returns "", "".
func scanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	// ETag is either W/"text" or "text".
	// See RFC 7232 2.3.
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		// Character values allowed in ETags.
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

// etagWeakMatch reports whether a and b match using weak ETag comparison.
// Assumes a and b are valid ETags.
func etagWeakMatch(a, b string) bool {
	return strings.TrimPrefix(a, "W/") == strings.TrimPrefix(b, "W/")
}

func writeNotModified(w http.ResponseWriter) {
	// RFC 7232 section 4.1:
	// a sender SHOULD NOT generate representation metadata other than the
	// above listed fields unless said metadata exists for the purpose of
	// guiding cache updates (e.g., Last-Modified might be useful if the
	// response does not have an ETag field).
	h := w.Header()
	delete(h, "Content-Type")
	delete(h, "Content-Length")
	delete(h, "Content-Encoding")
	if h.Get("Etag") != "" {
		delete(h, "Last-Modified")
	}
	w.WriteHeader(http.StatusNotModified)
}
