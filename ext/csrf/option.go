package csrf

import (
	"time"

	"github.com/yaitoo/xun"
)

type Options struct {
	SecretKey  []byte
	CookieName string
	MaxAge     int
	ExpireFunc func(*xun.Context) (bool, time.Duration)
}

type Option func(o *Options)

func WithCookie(name string) Option {
	return func(o *Options) {
		if name != "" {
			o.CookieName = name
		}
	}
}

func WithExpire(t time.Duration) Option {
	return func(o *Options) {
		if t > 0 {
			o.MaxAge = int(t / time.Second)
		}
	}
}

func WithExpireFunc(f func(*xun.Context) (bool, time.Duration)) Option {
	return func(o *Options) {
		o.ExpireFunc = f
	}
}
