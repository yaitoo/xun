package reqlog

import (
	"log"

	"github.com/yaitoo/xun"
)

// Miss is a default function that returns an empty string.
// It is used as a default argument for the WithVisitor and WithUser functions.
var Miss = func(*xun.Context) string { return "-" }

// Options represents the configuration options for the RequestLog middleware.
// It allows customizing the request log message format, the logger instance,
// and the functions to retrieve visitor and user information from the request context.
type Options struct {
	Logger     *log.Logger
	GetVisitor func(c *xun.Context) string
	GetUser    func(c *xun.Context) string
	Format     Format
	SkipFunc   func(c *xun.Context) bool
}

// Option is a function that takes a pointer to Options and modifies it.
// It is used to customize the behavior of the RequestLog middleware.
type Option func(o *Options)

// WithLogger sets the logger for the RequestLog middleware. If not set,
// it will use the package-level logger from the log package.
func WithLogger(l *log.Logger) Option {
	return func(o *Options) {
		if l != nil {
			o.Logger = l
		}
	}
}

// WithVisitor sets a custom function to retrieve visitor information from the request context.
// It will be used to populate the visitor field in the request log message.
//
// The function should take a pointer to the xun.Context and return a string.
// The empty string will be replaced with a dash in the log message.
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

// WithUser sets a custom function to retrieve user information from the request context.
// It will be used to populate the user field in the request log message.
//
// The function should take a pointer to the xun.Context and return a string.
// The empty string will be replaced with a dash in the log message.
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

// WithFormat sets a custom format for the request log message.
func WithFormat(f Format) Option {
	return func(o *Options) {
		if f != nil {
			o.Format = f
		}
	}
}

// WithSkip sets a custom function to skip the request log message.
// The function should take a pointer to the xun.Context and return a boolean.
// If the function returns true, the request log message will be skipped.
func WithSkip(f func(c *xun.Context) bool) Option {
	return func(o *Options) {
		o.SkipFunc = f
	}
}
