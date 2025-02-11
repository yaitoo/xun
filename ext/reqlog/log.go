package reqlog

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/yaitoo/xun"
)

func Logging(opts ...Option) xun.Middleware {
	options := &Options{
		Logger: log.Default(),
		GetVisitor: func(*http.Request) string {
			return "-"
		},
		GetUser: func(*http.Request) string {
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

				//远程主机IP、用户标识（通常为“-”）、认证用户名（如果有）、日期时间、请求行、HTTP状态码、返回给客户端的对象大小、引用页（referer）、用户代理（user-agent）。
				options.Logger.Printf(`%s %s %s %s %s %d %d "%s" "%s"\n`,
					host,
					options.GetVisitor(c.Request),
					options.GetUser(c.Request),
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
