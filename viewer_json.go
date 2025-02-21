package xun

import (
	"net/http"
)

// JsonViewer is a viewer that writes the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
type JsonViewer struct {
}

var jsonViewerMime = &MimeType{Type: "application", SubType: "json"}

// MimeType returns the MIME type of the JSON content.
//
// It returns "application/json".
func (*JsonViewer) MimeType() *MimeType {
	return jsonViewerMime
}

// Render renders the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
func (*JsonViewer) Render(ctx *Context, data any) error { // skipcq: RVV-B0012
	var err error
	ctx.Response.Header().Set("Content-Type", "application/json")
	if ctx.Request.Method != http.MethodHead {
		buf := BufPool.Get()
		defer BufPool.Put(buf)

		err = Json.NewEncoder(buf).Encode(data)
		if err != nil {
			return err
		}
		_, err = buf.WriteTo(ctx.Response)
	}

	return err
}
