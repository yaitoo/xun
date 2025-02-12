package reqlog

import (
	"log"
	"time"

	"github.com/yaitoo/xun"
)

// New returns a middleware that logs each incoming request to the provided
// logger. The format of the log messages is customizable using the Format
// option. The default format is the combined log format (XLF/ELF).
func New(opts ...Option) xun.Middleware {
	options := &Options{
		Logger:     log.Default(),
		GetVisitor: Miss,
		GetUser:    Miss,
		Format:     Combined,
	}

	for _, opt := range opts {
		opt(options)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			now := time.Now()
			defer func() {
				options.Format(c, options, now)
			}()
			return next(c)
		}
	}
}
