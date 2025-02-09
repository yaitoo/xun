package xun

import (
	"net/http"
	"strings"
)

// Context is the primary structure for handling HTTP requests.
// It encapsulates the request, response, routing information, and application context.
// It offers various methods to work with request data, manipulate responses, and manage routing.
type Context struct {
	Routing  Routing
	app      *App
	Response http.ResponseWriter
	Request  *http.Request

	statusCode int
	values     map[string]any
}

// WriteStatus sets the HTTP status code for the response.
// It is used to return error or success status codes to the client.
// The status code will be sent to the client only once the response body is closed.
// If a status code is not set, the default status code is 200 (OK).
func (c *Context) WriteStatus(code int) {
	if c.statusCode == 0 {
		c.Response.WriteHeader(code)
		c.statusCode = code
	}
}

// StatusCode returns the current HTTP status code for the response.
// If no status code has been explicitly set, it defaults to 200 (OK).
func (c *Context) StatusCode() int {
	if c.statusCode == 0 {
		return http.StatusOK
	}
	return c.statusCode
}

// WriteHeader sets a response header.
//
// If the value is an empty string, the header will be deleted.
func (c *Context) WriteHeader(key string, value string) {
	if value == "" {
		c.Response.Header().Del(key)
		return
	}

	c.Response.Header().Set(key, value)
}

// View renders the specified data as a response to the client.
// It can be used to render HTML, JSON, XML, or any other type of response.
//
// The first argument is the data to be rendered. The second argument is an
// optional list of viewer names. If the list is empty, the viewer associated
// with the current route will be used. If the list is not empty, the first
// viewer in the list that matches the current request will be used.
func (c *Context) View(data any, options ...string) error {
	var name string
	if len(options) > 0 {
		name = options[0]
	}

	v, ok := c.getViewer(name)

	if !ok {
		for _, accept := range c.Accept() {
			for _, viewer := range c.Routing.Viewers {
				if viewer.MimeType().Match(accept) {
					v = viewer
					ok = true
					break
				}
			}

			if ok {
				break
			}
		}
	}
	// no any viewer is matched
	if !ok {
		if v == nil {
			if len(c.Routing.Viewers) == 0 {
				return ErrViewNotFound
			}
			v = c.Routing.Viewers[0] // use the first viewer as a fallback when no viewer is matched or specified by name
		}
	}

	return v.Render(c.Response, c.Request, data)
}

// getViewer get viewer by name
func (c *Context) getViewer(name string) (Viewer, bool) {
	if name == "" {
		return nil, false
	}
	v, ok := c.app.viewers[name]
	if ok {
		mime := v.MimeType()
		for _, accept := range c.Accept() {
			if mime.Match(accept) {
				return v, true
			}
		}
	}
	return v, false
}

// Redirect redirects the user to the given url.
// It uses the given status code. If the status code is not provided,
// it uses http.StatusFound (302).
func (c *Context) Redirect(url string, statusCode ...int) {
	if c.app.interceptor != nil {
		if c.app.interceptor.Redirect(c, url, statusCode...) {
			return
		}

	}
	c.WriteHeader("Location", url)
	if len(statusCode) > 0 {
		c.WriteStatus(statusCode[0])
	} else {
		c.WriteStatus(http.StatusFound) // 302
	}

}

// AcceptLanguage returns a slice of strings representing the languages
// that the client accepts, in order of preference.
// The languages are normalized to lowercase and whitespace is trimmed.
func (c *Context) AcceptLanguage() (languages []string) {
	accepted := c.Request.Header.Get("Accept-Language")
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

// Accept returns a slice of strings representing the media types
// that the client accepts, in order of preference.
// The media types are normalized to lowercase and whitespace is trimmed.
func (c *Context) Accept() (types []MimeType) {
	accepted := c.Request.Header.Get("Accept")
	if accepted == "" {
		return
	}

	// text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7

	options := strings.Split(accepted, ",")
	l := len(options)
	types = make([]MimeType, l)

	for i := 0; i < l; i++ {
		if n := strings.IndexByte(options[i], ';'); n >= 0 {
			types[i] = NewMimeType(strings.TrimSpace(options[i][:n]))
		} else {
			types[i] = NewMimeType(strings.TrimSpace(options[i]))
		}
	}
	return
}

// RequestReferer returns the referer of the request.
func (c *Context) RequestReferer() string {
	var v string
	if c.app.interceptor != nil {
		v = c.app.interceptor.RequestReferer(c)
	}

	if v == "" {
		v = c.Request.Header.Get("Referer")
	}

	return v
}

// Get retrieves a value from the context's values map by key.
// If the values map is nil or the key does not exist, it returns nil.
func (c *Context) Get(key string) any {
	if c.values == nil {
		return nil
	}

	return c.values[key]
}

// Set assigns a value to the specified key in the context's values map.
// If the values map is nil, it initializes a new map.
func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}
