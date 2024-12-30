package htmx

import (
	"net/http"
	"net/url"
	"strings"
)

// Context is the primary structure for handling HTTP requests.
// It encapsulates the request, response, routing information, and application context.
// It offers various methods to work with request data, manipulate responses, and manage routing.
type Context struct {
	Routing Routing
	app     *App
	rw      http.ResponseWriter
	req     *http.Request

	writtenStatus bool
}

// Writer returns the http.ResponseWriter associated with the current context.
// It allows writing to the HTTP response body and setting response headers.
func (c *Context) Writer() http.ResponseWriter {
	return c.rw
}

// Request returns the HTTP request associated with the current context.
// It allows access to the request data and headers.
func (c *Context) Request() *http.Request {
	return c.req
}

// WriteStatus sets the HTTP status code for the response.
// It is used to return error or success status codes to the client.
// The status code will be sent to the client only once the response body is closed.
// If a status code is not set, the default status code is 200 (OK).
func (c *Context) WriteStatus(code int) {
	if !c.writtenStatus {
		c.rw.WriteHeader(code)
		c.writtenStatus = true
	}
}

// WriteHeader sets a response header.
//
// If the value is an empty string, the header will be deleted.
func (c *Context) WriteHeader(key string, value string) {
	if value == "" {
		c.rw.Header().Del(key)
		return
	}

	c.rw.Header().Set(key, value)
}

// View renders a view with the given data and optional view name.
// items should have 1 or 2 inputs. first one is data, second one is view name.
// If a view name is provided, it attempts to fetch a viewer by name and uses it to render the view.
// If no view name is provided, it uses the default viewer.
// The data parameter is any type and will be passed to the viewer's Render method.
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
		if ok {
			mime := v.MimeType()
			ok = false
			for _, accept := range c.Accept() {
				if mime == accept {
					ok = true
					break
				}
			}
		}

	} else {
		for _, accept := range c.Accept() {
			v, ok = c.Routing.Viewers[accept]
			if ok {
				break
			}
		}
	}

	if !ok {
		v = c.Routing.Options.viewer
	}

	if !c.writtenStatus {
		c.rw.WriteHeader(http.StatusOK)
	}

	return v.Render(c.rw, c.req, data)
}

// Redirect redirects the user to the given url.
// It uses the given status code. If the status code is not provided,
// it uses http.StatusFound (302).
func (c *Context) Redirect(url string, statusCode ...int) {

	if c.IsHxRequest() {
		c.WriteHeader("HX-Redirect", url)
		c.WriteStatus(http.StatusOK)
		return
	}

	c.WriteHeader("Location", url)
	if len(statusCode) > 0 {
		c.WriteStatus(statusCode[0])
	} else {
		c.WriteStatus(http.StatusFound) //302
	}

}

// AcceptLanguage returns a slice of strings representing the languages
// that the client accepts, in order of preference.
// The languages are normalized to lowercase and whitespace is trimmed.
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

// Accept returns a slice of strings representing the media types
// that the client accepts, in order of preference.
// The media types are normalized to lowercase and whitespace is trimmed.
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

// htmx helper

// IsHxRequest checks if the current request is an HTMX request.
// It returns true if the "HX-Request" header is set to "true".
func (c *Context) IsHxRequest() bool {
	return c.req.Header.Get(HxRequest) == "true"
}

// GetCurrentUrl returns the current URL of the request.
// If the request is an HTMX request, it returns the value of the "HX-Current-URL" header.
// Otherwise, it returns the request URL.
func (c *Context) GetCurrentUrl() *url.URL {
	if c.IsHxRequest() {
		u, _ := url.Parse(c.req.Header.Get(HxCurrentUrl))
		return u
	}

	return c.req.URL
}

// GetHeader returns the value of the specified header key.
// It is case-insensitive.
func (c *Context) GetHeader(key string) string {
	return c.req.Header.Get(key)
}

func (c *Context) WriteHtmxHeader(key string, value any) {
	buf, _ := json.Marshal(value)
	c.rw.Header().Set(key, string(buf))
}
