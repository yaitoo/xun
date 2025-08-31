package proxypass

import (
	"net"

	"github.com/yaitoo/xun"
)

func New(opts ...Option) xun.Middleware { // skipcq: GO-R1005
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	if options.GetVisitor == nil {
		options.GetVisitor = func(c *xun.Context) (string, string) {
			ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err != nil {
				return c.Request.RemoteAddr, ""
			}
			return ip, ""
		}
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {

			it := getReverseProxy(c, options)
			if it == nil {
				return next(c)
			}

			it.ServeHTTP(c.Response, c.Request)

			return nil
		}
	}
}
