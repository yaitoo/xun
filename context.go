package xun

import (
	"net/http"
	"strings"
)

type TempData map[string]any

// Context is the primary structure for handling HTTP requests.
// It encapsulates the request, response, routing information, and application context.
// It offers various methods to work with request data, manipulate responses, and manage routing.
type Context struct {
	Routing  Routing
	App      *App
	Response ResponseWriter
	Request  *http.Request

	TempData TempData

	// accepts / languages cache the parsed result of Accept() /
	// AcceptLanguage() for the lifetime of the request. Context is per
	// request (see app.go), so these fields never leak across requests.
	accepts       []MimeType
	acceptsDone   bool
	languages     []string
	languagesDone bool
}

// WriteStatus sets the HTTP status code for the response.
// It is used to return error or success status codes to the client.
// The status code will be sent to the client only once the response body is closed.
// If a status code is not set, the default status code is 200 (OK).
func (c *Context) WriteStatus(code int) {
	c.Response.WriteHeader(code)
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

	return v.Render(c, data)
}

// getViewer get viewer by name
func (c *Context) getViewer(name string) (Viewer, bool) {
	if name == "" {
		return nil, false
	}
	// app.viewers is mutated by the hot-reload goroutine when WithWatch
	// is enabled; with WithWatch (dev-only) concurrent access during
	// reload is undefined behavior — see the WithWatch doc.
	v, ok := c.App.viewers[name]
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
	if c.App.interceptor != nil {
		if c.App.interceptor.Redirect(c, url, statusCode...) {
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
//
// The result is cached on the Context: subsequent calls return the same
// slice without re-parsing the Accept-Language header. Context is per
// request (see app.go), so the cache cannot leak across requests.
//
// The returned slice is owned by the Context. Mutating it (including
// in-place writes or appends that fit within the slice's capacity) will
// corrupt the cache for the rest of the request. If you need to modify
// the result, take a copy first.
//
// If the Accept-Language header is mutated after the first call, the
// second call still returns the cached parsed value; the new header is
// not re-parsed.
//
// Returns nil if the header is empty or contains no usable entries.
func (c *Context) AcceptLanguage() []string {
	if c.languagesDone {
		return c.languages
	}
	c.languagesDone = true

	accepted := c.Request.Header.Get("Accept-Language")
	if accepted == "" {
		return nil
	}
	options := strings.Split(accepted, ",")

	for _, opt := range options {
		locale := strings.SplitN(opt, ";", 2)
		lang := strings.TrimSpace(locale[0])
		if lang == "" {
			continue
		}
		c.languages = append(c.languages, strings.ToLower(lang))
	}
	return c.languages
}

// Accept returns a slice of MimeType representing the media types
// that the client accepts, in order of preference.
// The media types are normalized to lowercase and whitespace is trimmed.
//
// The result is cached on the Context: subsequent calls return the same
// slice without re-parsing the Accept header. Context is per request
// (see app.go), so the cache cannot leak across requests.
//
// The returned slice is owned by the Context. Mutating it (including
// in-place writes or appends that fit within the slice's capacity) will
// corrupt the cache for the rest of the request. If you need to modify
// the result, take a copy first.
//
// If the Accept header is mutated after the first call, the second call
// still returns the cached parsed value; the new header is not re-parsed.
//
// Returns nil if the header is empty or contains no usable entries.
func (c *Context) Accept() []MimeType {
	if c.acceptsDone {
		return c.accepts
	}
	c.acceptsDone = true

	accepted := c.Request.Header.Get("Accept")
	if accepted == "" {
		return nil
	}

	// text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7

	options := strings.Split(accepted, ",")

	for _, opt := range options {
		if n := strings.IndexByte(opt, ';'); n >= 0 {
			opt = opt[:n]
		}
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		c.accepts = append(c.accepts, NewMimeType(strings.ToLower(opt)))
	}
	return c.accepts
}

// RequestReferer returns the referer of the request.
func (c *Context) RequestReferer() string {
	var v string
	if c.App.interceptor != nil {
		v = c.App.interceptor.RequestReferer(c)
	}

	if v == "" {
		v = c.Request.Header.Get("Referer")
	}

	return v
}

// Get retrieves a value from the context's TempData by key.
func (c *Context) Get(key string) any {
	return c.TempData[key]
}

// Set assigns a value to the specified key in the context's TempData.
func (c *Context) Set(key string, value any) {
	c.TempData[key] = value
}
