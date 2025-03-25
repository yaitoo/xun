package cache

import (
	"strconv"

	"github.com/yaitoo/xun"
)

func New(opts ...Option) xun.Middleware { // skipcq: GO-R1005
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			for _, rule := range options.Rules {
				if rule.Match(c.Request.URL.Path) {
					c.WriteHeader("Cache-Control", "max-age="+strconv.Itoa(int(rule.Duration.Seconds())))
					break
				}
			}

			return next(c)
		}
	}
}
