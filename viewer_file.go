package htmx

import (
	"io/fs"
	"net/http"
)

// FileViewer is a viewer that serves a file from a file system.
//
// You can use it to serve a file from a file system, or to serve a file from
// a zip file.
//
// The file system is specified by the `fsys` field, and the path is specified
// by the `path` field.
//
// For example, to serve a file from the current working directory, you can
// use the following code:
//
//	viewer := &FileViewer{
//	    fsys: os.DirFS("."),
//	    path: "example.txt",
//	}
//
//	app.HandleFile("example.txt", viewer)
type FileViewer struct {
	fsys fs.FS
	path string
}

// MimeType returns the MIME type of the file.
//
// The MIME type is determined by the file extension of the file.
func (*FileViewer) MimeType() string {
	return "*/*"
}

// Render serves a file from the file system using the FileViewer.
// It writes the file to the http.ResponseWriter.
func (v *FileViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	http.ServeFileFS(w, r, v.fsys, v.path)
	return nil
}
