package csrf

const (
	DefaultCookieName = "csrf_token"
)

// Options represents the configuration for the CSRF middleware.
// It allows customizing the secret key, cookie name, maximum age,
// and an expiration function for the CSRF token.
type Options struct {
	SecretKey  []byte
	CookieName string
	JsToken    bool
}

// Option is a function type that takes a pointer to Options and modifies it.
// It is used to customize the behavior of the CSRF middleware.
type Option func(o *Options)

// WithCookie sets the name of the cookie to use for storing the CSRF token.
// Defaults to "csrf_token".
func WithCookie(name string) Option {
	return func(o *Options) {
		if name != "" {
			o.CookieName = name
		}
	}
}

// WithJsToken enables the JavaScript token feature for CSRF protection.
//
// It sets the JsToken field in the Options struct to true, allowing the
// middleware to generate and handle CSRF tokens via JavaScript.
func WithJsToken() Option {
	return func(o *Options) {
		o.JsToken = true
	}
}
