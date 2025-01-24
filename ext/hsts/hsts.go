// Package hsts provides functionality for handling HTTP Strict Transport Security (HSTS).
//
// This package includes options and middleware for configuring HSTS headers in HTTP responses.
// HSTS is a web security policy mechanism that helps protect websites against man-in-the-middle
// attacks such as protocol downgrade attacks and cookie hijacking.
//
// The primary components of this package are:
// - Config: A struct representing HSTS configuration options like MaxAge, IncludeSubDomains, and Preload.
// - Option: A function type used to modify a Config instance with different settings.
// - Middleware: A function to apply HSTS settings to HTTP responses.
//
// Example usage:
//   app := xun.New()
//   app.Use(hsts.Header(hsts.WithMaxAge(24 * time.Hour), hsts.WithPreload(true)))
//
// This would set the HSTS header with a max-age of 1 day and enable preload for the domain.

package hsts

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/yaitoo/xun"
)

const defaultMaxAge = int64(365 * 24 * time.Hour / time.Second)

// WriteHeader is a middleware that sets the STS response header for a HTTPs request.
func WriteHeader(opts ...Option) xun.Middleware {
	cfg := &Config{
		MaxAge:            defaultMaxAge,
		IncludeSubDomains: false,
		Preload:           false,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			r := c.Request()

			if r.TLS != nil && (r.Method == "GET" || r.Method == "HEAD") {
				v := "max-age=" + strconv.FormatInt(cfg.MaxAge, 10)
				if cfg.IncludeSubDomains {
					v += "; includeSubDomains"
				}
				if cfg.Preload {
					v += "; preload"
				}
				c.WriteHeader("Strict-Transport-Security", v)
			}

			return next(c)
		}
	}
}

// Redirect is a middleware that redirects plain HTTP requests to HTTPS.
func Redirect() xun.Middleware {
	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			r := c.Request()

			if r.TLS == nil && (r.Method == "GET" || r.Method == "HEAD") {
				target := "https://" + stripPort(r.Host) + r.URL.RequestURI()

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
