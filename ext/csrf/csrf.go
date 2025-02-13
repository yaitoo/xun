package csrf

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"net/http"
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

var lastModified = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

func LoadJsToken(opts ...Option) xun.HandleFunc {
	o := &Options{
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

	return func(c *xun.Context) error {
		content := bytes.NewReader(processed.Bytes())
		c.Response.Header().Set("Content-Type", "application/javascript")
		http.ServeContent(c.Response, c.Request, "csrf.js", lastModified, content)
		return nil
	}
}
