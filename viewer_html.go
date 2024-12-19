package htmx

import (
	"net/http"
)

var bufpool *BufferPool

func init() {
	bufpool = NewBufferPool(100)
}

type HtmlViewer struct {
	template *Template
}

func (*HtmlViewer) MimeType() string {
	return "text/html"
}

func (v *HtmlViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	err := v.template.Execute(buf, data)
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(w)
	return err
}
