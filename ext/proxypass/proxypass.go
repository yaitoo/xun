package proxypass

import (
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yaitoo/xun"
	"github.com/yaitoo/xun/ext/sse"
)

var (
	connections *sse.Server
	servers     *atomic.Value
	mu          sync.Mutex
)

func New(opts ...Option) xun.Middleware { // skipcq: GO-R1005
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	connections = sse.New()

	servers = &atomic.Value{}
	servers.Store(make(map[string]*Proxy))

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			host, _, _ := net.SplitHostPort(c.Request.Host)
			host = strings.ToLower(host)
			it := get(host, strings.TrimPrefix(host, "www."))

			if it == nil {
				return next(c)
			}

			it.ServeHTTP(c.Response, c.Request)

			return nil
		}
	}
}
