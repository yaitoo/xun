package htmx

import "github.com/yaitoo/xun"

// IsHxRequest reports whether the current request is an HTMX request.
// It returns true when the "HX-Request" header equals "true".
//
// This is a convenience helper that mirrors (*interceptor).IsHxRequest
// without requiring callers to construct an interceptor instance. It
// lets handlers branch on HTMX behavior without registering
// xun.WithInterceptor, while still using the same header check that
// the interceptor performs.
func IsHxRequest(c *xun.Context) bool {
	return c.Request.Header.Get(HxRequest) == "true"
}

// IsBoosted reports whether the request was issued via an element that
// uses hx-boost.
func IsBoosted(c *xun.Context) bool {
	return c.Request.Header.Get(HxBoosted) == "true"
}

// IsHistoryRestore reports whether the request was triggered by
// htmx's local history cache restoration after a miss.
func IsHistoryRestore(c *xun.Context) bool {
	return c.Request.Header.Get(HxHistoryRestoreRequest) == "true"
}

// Target returns the id of the target element sent by htmx, or an
// empty string if it is not present.
func Target(c *xun.Context) string {
	return c.Request.Header.Get(HxTarget)
}

// Trigger returns the id of the triggered element sent by htmx, or
// an empty string if it is not present.
func Trigger(c *xun.Context) string {
	return c.Request.Header.Get(HxTrigger)
}

// TriggerName returns the name attribute of the triggered element
// sent by htmx, or an empty string if it is not present.
func TriggerName(c *xun.Context) string {
	return c.Request.Header.Get(HxTriggerName)
}

// Prompt returns the user response to an hx-prompt, or an empty
// string if no prompt was shown.
func Prompt(c *xun.Context) string {
	return c.Request.Header.Get(HxPrompt)
}

// CurrentURL returns the current URL of the browser as reported by
// htmx via the "HX-Current-Url" header, or an empty string when the
// header is missing. This is the htmx-aware equivalent of the
// Referer header for HTMX requests.
func CurrentURL(c *xun.Context) string {
	return c.Request.Header.Get(HxCurrentUrl)
}
