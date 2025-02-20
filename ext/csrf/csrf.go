package csrf

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
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

	buf := loadJavaScript(o)

	mac := hmac.New(sha256.New, o.SecretKey)
	etag := xun.ComputeETagWith(bytes.NewReader(buf), mac)

	return func(c *xun.Context) error {
		c.Response.Header().Set("ETag", etag)
		if xun.WriteIfNoneMatch(c.Response, c.Request) {
			return nil
		}

		content := bytes.NewReader(buf)
		c.Response.Header().Set("Content-Type", "application/javascript")
		http.ServeContent(c.Response, c.Request, "csrf.js", zeroTime, content)

		return nil
	}
}

func loadJavaScript(o *Options) []byte {
	f, _ := fsys.Open("csrf.js") // nolint: errcheck
	defer f.Close()

	buf, _ := io.ReadAll(f) // nolint: errcheck

	t, _ := template.New("token").Parse(string(buf)) // nolint: errcheck

	var body bytes.Buffer
	t.Execute(&body, o) // nolint: errcheck

	return body.Bytes()
}
