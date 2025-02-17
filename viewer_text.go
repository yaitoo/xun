package xun

import (
	"net/http"
)

func NewTextViewer(t *TextTemplate) *TextViewer {
	return &TextViewer{template: t}
}

// TextViewer is a struct that holds an TextTemplate and is used to render text content.
type TextViewer struct {
	template *TextTemplate
}

// MimeType returns the MIME type for the text content rendered by the TextViewer.
func (v *TextViewer) MimeType() *MimeType {
	return &v.template.mime
}

// Render writes the text content rendered by the TextViewer to the provided http.ResponseWriter.
// It sets the Content-Type header to "text/plain; charset=utf-8" and writes the rendered content to the response.
// If there is an error executing the template, it is returned.
func (v *TextViewer) Render(ctx *Context, data any) error { // skipcq: RVV-B0012
	var err error
	ctx.Response.Header().Set("Content-Type", v.template.mime.String()+v.template.charset)
	if ctx.Request.Method != http.MethodHead {
		buf := BufPool.Get()
		defer BufPool.Put(buf)

		err = v.template.Execute(buf, data)
		if err != nil {
			return err
		}

		_, err = buf.WriteTo(ctx.Response)
	}

	return err
}
