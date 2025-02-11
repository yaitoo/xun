package reqlog

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/yaitoo/xun"
)

func Logging(opts ...Option) xun.Middleware {
	options := &Options{
		Logger: log.Default(),
		GetVisitor: func(c *xun.Context) string {
			return "-"
		},
		GetUser: func(c *xun.Context) string {
			return "-"
		},
	}

	for _, opt := range opts {
		opt(options)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			now := time.Now()
			defer func() {

				requestLine := fmt.Sprintf(`"%s %s %s"`, c.Request.Method, c.Request.URL.Path, c.Request.Proto)
				host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)

				//COMBINED: remote、visitor、user、datetime、request line、status、body_bytes_sent、referer、user-agent
				options.Logger.Printf("%s %s %s %s %s %d %d \"%s\" \"%s\"\n",
					host,
					options.GetVisitor(c),
					options.GetUser(c),
					now.Format("[02/Jan/2006:15:04:05 -0700]"),
					requestLine,
					c.Response.StatusCode(),
					c.Response.BodyBytesSent(),
					c.Request.Referer(),
					c.Request.UserAgent(),
				)
			}()
			return next(c)
		}
	}
}
