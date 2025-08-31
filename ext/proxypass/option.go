package proxypass

import "github.com/yaitoo/xun"

type Options struct {
	GetVisitor func(c *xun.Context) (string, string)
}

type Option func(o *Options)

func WithForwarder(c Forwarder) Option {
	return func(o *Options) {
		o.GetVisitor = func(ctx *xun.Context) (string, string) {
			return c.GetVisitor(ctx)
		}
	}
}
