package xun

import (
	"net/http"
)

// TextViewer is a struct that holds an HtmlTemplate and is used to render text content.
type TextViewer struct {
	template *TextTemplate
}

// MimeType returns the MIME type for the text content rendered by the TextViewer.
func (*TextViewer) MimeType() string {
	return "text/plain"
}

// Render writes the text content rendered by the TextViewer to the provided http.ResponseWriter.
// It sets the Content-Type header to "text/plain; charset=utf-8" and writes the rendered content to the response.
// If there is an error executing the template, it is returned.
func (v *TextViewer) Render(w http.ResponseWriter, r *http.Request, data any) error { //skipcq: RVV-B0012
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	buf := BufPool.Get()
	defer BufPool.Put(buf)

	err := v.template.Execute(buf, data)
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(w)
	return err
}
