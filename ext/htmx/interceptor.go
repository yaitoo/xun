package htmx

import (
	"net/http"

	"github.com/yaitoo/xun"
)

// New creates a new HTMX interceptor.
func New() xun.Interceptor {
	return &interceptor{}
}

type interceptor struct {
}

// IsHxRequest checks if the current request is an HTMX request.
// It returns true if the "HX-Request" header is set to "true".
func (i *interceptor) IsHxRequest(c *xun.Context) bool {
	return c.Request().Header.Get(HxRequest) == "true"
}

// RequestReferer returns the referer of the request.
func (i *interceptor) RequestReferer(c *xun.Context) string {
	if i.IsHxRequest(c) {
		return c.Request().Header.Get(HxCurrentUrl)
	}

	return ""
}

func (i *interceptor) Redirect(c *xun.Context, url string, statusCode ...int) bool {
	if i.IsHxRequest(c) {
		c.WriteHeader("HX-Redirect", url)
		c.WriteStatus(http.StatusOK)

		return true
	}
	return false
}
