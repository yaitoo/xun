package reqlog

import (
	"log"

	"github.com/yaitoo/xun"
)

type Options struct {
	Logger     *log.Logger
	GetVisitor func(c *xun.Context) string
	GetUser    func(c *xun.Context) string
}

type Option func(o *Options)

func WithLogger(l *log.Logger) Option {
	return func(o *Options) {
		if l != nil {
			o.Logger = l
		}
	}
}

func WithVisitor(get func(c *xun.Context) string) Option {
	return func(o *Options) {
		if get != nil {
			o.GetVisitor = func(c *xun.Context) string {
				v := get(c)
				if v == "" {
					return "-"
				}

				return v
			}
		}

	}
}

func WithUser(get func(c *xun.Context) string) Option {
	return func(o *Options) {
		if get != nil {
			o.GetUser = func(c *xun.Context) string {

				u := get(c)
				if u == "" {
					return "-"
				}

				return u
			}
		}
	}
}
