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
func (t *TextTemplate) Load(fsys fs.FS, templates map[string]*TextTemplate) error {
	buf, err := fs.ReadFile(fsys, t.name)
	if err != nil {
		return err
	}

	nt := template.New(t.name).Funcs(FuncMap)

	charset := "; charset=utf-8"
	mt := "text/plain"

	defer func() {
		t.template = nt

		// text/plain; charset=utf-8
		i := strings.Index(mt, ";")
		if i > -1 {
			charset = mt[i:]
			mt = mt[:i]
		}

		t.charset = charset
		t.mime = mt
	}()

	if len(buf) == 0 {
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

	return nil
}

// Reload reloads the template and all its dependents from the given file system.
func (t *TextTemplate) Reload(fsys fs.FS, templates map[string]*TextTemplate) error {
	err := t.Load(fsys, templates)
	if err != nil {
		return err
	}

	return nil
}

func (t *TextTemplate) Execute(wr io.Writer, data any) error {
	return t.template.Execute(wr, data)
}
