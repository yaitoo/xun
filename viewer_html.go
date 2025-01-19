package xun

import (
	"net/http"
)

// HtmlViewer is a viewer that renders a html template.
//
// It uses the `HtmlTemplate` type to render a template.
// The template is loaded from the file system when the viewer is created.
// The `Render` method renders the template with the given data and writes the
// result to the http.ResponseWriter.
type HtmlViewer struct {
	template *HtmlTemplate
}

var htmlViewerMime = &MimeType{Type: "text", SubType: "html"}

// MimeType returns the MIME type of the HTML content.
//
// This implementation returns "text/html".
func (*HtmlViewer) MimeType() *MimeType {
	return htmlViewerMime
}

// Render renders the template with the given data and writes the result to the http.ResponseWriter.
//
// This implementation uses the `HtmlTemplate.Execute` method to render the template.
// The rendered result is written to the http.ResponseWriter.
func (v *HtmlViewer) Render(w http.ResponseWriter, r *http.Request, data any) error { // skipcq: RVV-B0012
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	buf := BufPool.Get()
	defer BufPool.Put(buf)

	err := v.template.Execute(buf, data)
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(w)
	return err
}
