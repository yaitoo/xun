// Package cache provides HTTP caching middleware for xun web applications.
// It allows setting Cache-Control headers based on configurable rules and paths.
package cache

import (
	"strconv"

	"github.com/yaitoo/xun"
)

// New creates a xun middleware that applies caching rules to HTTP responses.
// It accepts optional configurations through Option functions and returns
// a middleware that sets Cache-Control headers based on request URL paths.
func New(opts ...Option) xun.Middleware { // skipcq: GO-R1005
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			// Apply caching rules based on request path
			for _, rule := range options.Rules {
				if rule.Match(c.Request.URL.Path) {
					c.WriteHeader("Cache-Control", "public, max-age="+strconv.Itoa(int(rule.Duration.Seconds())))
					break
				}
			}

			return next(c)
		}
	}
}
