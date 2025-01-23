package hsts

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/yaitoo/xun"
)

// Enable sets the Strict-Transport-Security header with the given maxAge,
// includeSubdomains and preload values.
//
// The Strict-Transport-Security header is used to inform browsers that the site
// should only be accessed over HTTPS, and that any HTTP requests should be
// automatically rewritten as HTTPS.
//
// maxAge is the maximum age of the header in seconds.
//
// includeSubdomains will include all subdomains of the current domain in the
// header.
//
// preload will add the preload directive to the header, which allows the site
// to be included in the HSTS preload list.
//
// The HSTS preload list is a list of sites that are known to be HTTPS-only, and
// are included in the browser's HSTS list by default. This allows the browser to
// immediately switch to HTTPS for these sites, without having to wait for the
// first request to complete.
func Enable(maxAge time.Duration, includeSubdomains, preload bool) xun.Middleware {
	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			r := c.Request()

			isHTTPS := false
			// Check X-Forwarded-Proto header first
			forwardedProto := r.Header.Get("X-Forwarded-Proto")
			if forwardedProto != "" {
				isHTTPS = forwardedProto == "https"
			} else {
				// Fall back to checking direct protocol
				isHTTPS = r.TLS != nil
			}

			if !isHTTPS && (r.Method == "GET" || r.Method == "HEAD") {
				target := "https://" + stripPort(r.Host) + r.URL.RequestURI()

				if maxAge <= 0 {
					maxAge = 365 * 24 * time.Hour
				}

				v := "max-age=" + strconv.FormatInt(int64(maxAge/time.Second), 10)
				if includeSubdomains {
					v += "; includeSubDomains"
				}
				if preload {
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
