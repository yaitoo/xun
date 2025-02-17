package xun

import (
	"fmt"
	"net/http"
)

// StringViewer is a viewer that writes the given data as string to the http.ResponseWriter.
//
// It sets the Content-Type header to "text/plain".
type StringViewer struct {
}

var StringViewerMime = &MimeType{Type: "text", SubType: "plain"}

// MimeType returns the MIME type of the string content.
//
// It returns "text/plain".
func (*StringViewer) MimeType() *MimeType {
	return StringViewerMime
}

// Render renders the given data as string to the http.ResponseWriter.
//
// It sets the Content-Type header to "text/plain; charset=utf-8".
func (*StringViewer) Render(ctx *Context, data any) error { // skipcq: RVV-B0012
	var err error
	ctx.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if data == nil {
		return nil
	}

	if ctx.Request.Method != http.MethodHead {
		_, err = fmt.Fprint(ctx.Response, data)
	}

	return err
}
