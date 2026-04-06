package proxypass

type Options struct {
	// GetVisitor func(c *xun.Context) (string, string)
}

type Option func(o *Options)

func WithForwarder(c ...Forwarder) Option {
	return func(o *Options) {
		for _, it := range c {
			RegisterForwarder(it.Name(), it)
		}
	}
}
