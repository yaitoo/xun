package xun

// Interceptor is an interface that provides methods to intercept requests and response.
type Interceptor interface {
	// RequestReferer returns the referer of the request.
	RequestReferer(c *Context) string

	// Redirect sends an HTTP redirect to the client.
	Redirect(c *Context, url string, statusCode ...int) bool
}
