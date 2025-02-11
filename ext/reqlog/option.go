package reqlog

import (
	"log"
	"net/http"
)

type Options struct {
	Logger     *log.Logger
	GetVisitor func(r *http.Request) string
	GetUser    func(r *http.Request) string
}

type Option func(o *Options)

func WithLogger(l *log.Logger) Option {
	return func(o *Options) {
		if l != nil {
			o.Logger = l
		}
	}
}

func WithVisitor(get func(r *http.Request) string) Option {
	return func(o *Options) {
		if get != nil {
			o.GetVisitor = func(r *http.Request) string {
				v := get(r)
				if v == "" {
					return "-"
				}

				return v
			}
		}

	}
}

func WithUser(get func(r *http.Request) string) Option {
	return func(o *Options) {
		if get != nil {
			o.GetUser = func(r *http.Request) string {

				u := get(r)
				if u == "" {
					return "-"
				}

				return u
			}
		}
	}
}
