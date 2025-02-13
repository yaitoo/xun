package csrf

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yaitoo/xun"
)

// New returns a middleware that generates a CSRF token and validates it in the request.
//
// The opts parameter is a list of Option functions that can be used to customize the
// behavior of the CSRF middleware. See the Option type for more information.
func New(secretKey []byte, opts ...Option) xun.Middleware {
	o := &Options{
		SecretKey:  secretKey,
		CookieName: DefaultCookieName,
	}

	for _, opt := range opts {
		opt(o)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {

			token, _ := c.Request.Cookie(o.CookieName) // nolint: errcheck

			if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
				if token == nil { // csrf_token doesn't exists
					setTokenCookie(c, o)
				}

				return next(c)
			}

			if !verifyToken(token, c.Request, o) {
				c.WriteStatus(http.StatusTeapot)
				return xun.ErrCancelled
			}

			return next(c)
		}
	}
}

//go:embed csrf.js
var fsys embed.FS

var zeroTime time.Time

// HandleFunc serves the JavaScript token required for the CSRF middleware.
//
// It takes the secret key and options to customize the behavior. See the Option
// type for more information.
func HandleFunc(secretKey []byte, opts ...Option) xun.HandleFunc {
	o := &Options{
		SecretKey:  secretKey,
		CookieName: DefaultCookieName,
	}
	for _, opt := range opts {
		opt(o)
	}

	f, _ := fsys.Open("csrf.js") // nolint: errcheck
	defer f.Close()

	buf, _ := io.ReadAll(f) // nolint: errcheck

	t, _ := template.New("token").Parse(string(buf)) // nolint: errcheck

	var processed bytes.Buffer
	t.Execute(&processed, o) // nolint: errcheck

	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(processed.Bytes())

	etag := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return func(c *xun.Context) error {
		if match := c.Request.Header.Get("If-None-Match"); match != "" {
			for _, it := range strings.Split(match, ",") {
				if strings.TrimSpace(it) == etag {
					c.Response.WriteHeader(http.StatusNotModified)
					return nil
				}
			}
		}

		c.Response.Header().Set("ETag", etag)

		content := bytes.NewReader(processed.Bytes())
		c.Response.Header().Set("Content-Type", "application/javascript")
		http.ServeContent(c.Response, c.Request, "csrf.js", zeroTime, content)
		return nil
	}
}
