package xun

import (
	"encoding/xml"
	"net/http"
)

// XmlViewer is a viewer that writes the given data as xml to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/xml".
type XmlViewer struct {
}

var XmlViewerMime = &MimeType{Type: "text", SubType: "xml"}

// MimeType returns the MIME type of the xml content.
//
// It returns "text/xml".
func (*XmlViewer) MimeType() *MimeType {
	return XmlViewerMime
}

// Render renders the given data as xml to the http.ResponseWriter.
//
// It sets the Content-Type header to "text/xml; charset=utf-8".
func (*XmlViewer) Render(ctx *Context, data any) error { // skipcq: RVV-B0012
	var err error
	ctx.Response.Header().Set("Content-Type", "text/xml; charset=utf-8")
	if ctx.Request.Method != http.MethodHead {
		buf := BufPool.Get()
		defer BufPool.Put(buf)

		err = xml.NewEncoder(buf).Encode(data)
		if err != nil {
			return err
		}
		_, err = buf.WriteTo(ctx.Response)
	}

	return err
}
