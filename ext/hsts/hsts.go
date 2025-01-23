package hsts

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/yaitoo/xun"
)

const defaultMaxAge = int64(365 * 24 * time.Hour / time.Second)

// Enable sets the Strict-Transport-Security header with the given maxAge,
// includeSubdomains and preload values.
//
// The Strict-Transport-Security header is used to inform browsers that the site
// should only be accessed over HTTPS, and that any HTTP requests should be
// automatically rewritten as HTTPS.
func Enable(opts ...Option) xun.Middleware {
	cfg := &Config{
		MaxAge:            defaultMaxAge,
		IncludeSubDomains: true,
		Preload:           true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			r := c.Request()

			if r.TLS == nil && (r.Method == "GET" || r.Method == "HEAD") {
				target := "https://" + stripPort(r.Host) + r.URL.RequestURI()

				v := "max-age=" + strconv.FormatInt(cfg.MaxAge, 10)
				if cfg.IncludeSubDomains {
					v += "; includeSubDomains"
				}
				if cfg.Preload {
					v += "; preload"
				}
				c.WriteHeader("Strict-Transport-Security", v)

				c.Redirect(target, http.StatusFound)
				return xun.ErrCancelled
			}

			return next(c)
		}
	}
}

func stripPort(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}
