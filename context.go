package htmx

import (
	"net/http"
	"strings"
)

// Context holds the state for an HTTP request, including the application instance,
// the HTTP request, the HTTP response writer, and a Viewer instance for rendering.
// It is used to manage and pass request-scoped data throughout the request lifecycle.
type Context struct {
	routing Routing
	app     *App
	rw      http.ResponseWriter
	req     *http.Request

	writtenStatus bool
}

// Writer returns the http.ResponseWriter that the Context is wrapping.
// It allows direct manipulation of the response writer for the current request.
func (c *Context) Writer() http.ResponseWriter {
	return c.rw
}

// Request returns the underlying HTTP request object associated with the context.
// It is an exported method, intended for use by other packages that need access
// to the request details from within the context.
func (c *Context) Request() *http.Request {
	return c.req
}

func (c *Context) WriteStatus(code int) {
	if !c.writtenStatus {
		c.rw.WriteHeader(code)
		c.writtenStatus = true
	}
}

// Header sets or deletes the header with the specified key and value.
// If the value is an empty string, the header will be deleted.
// Otherwise, the header will be set with the provided key-value pair.
func (c *Context) Header(key string, value string) {
	if value == "" {
		c.rw.Header().Del(key)
		return
	}

	c.rw.Header().Set(key, value)
}

func (c *Context) View(items ...any) error {

	var v Viewer
	var ok bool

	var data any
	var name string
	if len(items) > 0 {
		data = items[0]
	}

	if len(items) > 1 {
		name, _ = items[1].(string)
	}

	if name != "" {
		v, ok = c.app.viewers[name]
	} else {
		for _, accept := range c.Accept() {
			v, ok = c.routing.Viewers[accept]
			if ok {
				break
			}
		}
	}

	if !ok {
		v = c.routing.Options.viewer
	}

	if !c.writtenStatus {
		c.rw.WriteHeader(http.StatusOK)
	}

	return v.Render(c.rw, c.req, data)
}

// Redirect sends a redirect response to the client with the specified status code and URL.
// It sets the 'Location' header to the provided URL and writes the status code to the response writer.
// This function is an exported API of the Context struct, used for controlling the HTTP response flow.
func (c *Context) Redirect(statusCode int, url string) {
	c.rw.WriteHeader(statusCode)
	c.writtenStatus = true
	c.rw.Header().Set("Location", url)
}

func (c *Context) AcceptLanguage() (languages []string) {
	accepted := c.req.Header.Get("Accept-Language")
	if accepted == "" {
		return
	}
	options := strings.Split(accepted, ",")
	l := len(options)
	languages = make([]string, l)

	for i := 0; i < l; i++ {
		locale := strings.SplitN(options[i], ";", 2)
		languages[i] = strings.Trim(locale[0], " ")
	}
	return
}

func (c *Context) Accept() (types []string) {
	accepted := c.req.Header.Get("Accept")
	if accepted == "" {
		return
	}
	options := strings.Split(accepted, ",")
	l := len(options)
	types = make([]string, l)

	for i := 0; i < l; i++ {
		items := strings.SplitN(options[i], ";", 2)
		types[i] = strings.Trim(items[0], " ")
	}
	return
}
