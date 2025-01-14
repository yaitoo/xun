package xun

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
)

// NewTextTemplate creates a new TextTemplate with the given name and path.
func NewTextTemplate(name string) *TextTemplate {
	return &TextTemplate{
		name: name,
	}
}

// TextTemplate represents a text template that can be loaded from a file system and executed with data.
type TextTemplate struct {
	template *template.Template

	name    string
	mime    string
	charset string
}

// Load loads the template from the given file system.
func (t *TextTemplate) Load(fsys fs.FS) error {
	buf, err := fs.ReadFile(fsys, t.name)
	if err != nil {
		return err
	}

	nt := template.New(t.name).Funcs(FuncMap)
	charset := "; charset=utf-8"
	mt := "text/plain"

	defer func() {
		t.template = nt
		t.charset = charset
		t.mime = mt
	}()

	if len(buf) == 0 {
		nt, _ = nt.Parse("")
		return nil
	}

	mt = mime.TypeByExtension(filepath.Ext(t.name))
	if mt == "" {
		mt = http.DetectContentType(buf)
	}

	nt, err = nt.Parse(string(buf))
	if err != nil {
		return err
	}

	// text/plain; charset=utf-8
	i := strings.Index(mt, ";")
	if i > -1 {
		charset = mt[i:]
		mt = mt[:i]
	}

	return nil
}

// Reload reloads the template and all its dependents from the given file system.
func (t *TextTemplate) Reload(fsys fs.FS) error {
	err := t.Load(fsys)
	if err != nil {
		return err
	}

	return nil
}

func (t *TextTemplate) Execute(wr io.Writer, data any) error {
	return t.template.Execute(wr, data)
}
