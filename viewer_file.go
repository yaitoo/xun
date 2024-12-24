package htmx

import (
	"io/fs"
	"net/http"
)

type FileViewer struct {
	fsys fs.FS
	path string
}

func (*FileViewer) MimeType() string {
	return "*/*"
}

func (v *FileViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	http.ServeFileFS(w, r, v.fsys, v.path)
	return nil
}
