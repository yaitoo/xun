package xun

import (
	"crypto/sha512"
	"fmt"
	"io"
	"io/fs"
	"net/http"
)

// NewFileViewer creates a new FileViewer instance.
func NewFileViewer(fsys fs.FS, path string, isEmbed bool) *FileViewer {
	v := &FileViewer{
		fsys: fsys,
		path: path,
	}

	if isEmbed {
		f, err := fsys.Open(path)
		if err != nil {
			return v
		}
		defer f.Close()

		hash := sha512.New()
		if _, err := io.Copy(hash, f); err != nil {
			return v
		}
		v.isEmbed = true
		v.etag = fmt.Sprintf(`"%x"`, hash.Sum(nil))
	}

	return v
}

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

	isEmbed bool
	etag    string
}

var fileViewerMime = &MimeType{Type: "*", SubType: "*"}

// MimeType returns the MIME type of the file.
//
// The MIME type is determined by the file extension of the file.
func (*FileViewer) MimeType() *MimeType {
	return fileViewerMime
}

// Render serves a file from the file system using the FileViewer.
// It writes the file to the http.ResponseWriter.
func (v *FileViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	if !v.isEmbed {
		return v.serveContent(w, r)
	}

	w.Header().Set("ETag", v.etag)
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == v.etag {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}

	return v.serveContent(w, r)
}

func (v *FileViewer) serveContent(w http.ResponseWriter, r *http.Request) error {
	f, err := v.fsys.Open(v.path)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(w, r, v.path, fi.ModTime(), f.(io.ReadSeeker))

	return nil
}
