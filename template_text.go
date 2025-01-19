package xun

import (
	"io"
	"io/fs"
	"text/template"
)

// TextTemplate represents a text template that can be loaded from a file system and executed with data.
type TextTemplate struct {
	template *template.Template

	name    string
	mime    MimeType
	charset string
}

// Load loads the template from the given file system.
func (t *TextTemplate) Load(fsys fs.FS) error {
	buf, err := fs.ReadFile(fsys, t.name)
	if err != nil {
		return err
	}

	nt := template.New(t.name).Funcs(FuncMap)

	if len(buf) == 0 {
		nt, _ = nt.Parse("")
		t.template = nt
		t.mime = MimeType{Type: "text", SubType: "plain"}
		t.charset = "; charset=utf-8"
		return nil
	}

	nt, err = nt.Parse(string(buf))
	if err != nil {
		return err
	}

	t.mime, t.charset = GetMimeType(t.name, buf)
	t.template = nt

	return nil
}

// Reload reloads the template from the given file system.
func (t *TextTemplate) Reload(fsys fs.FS) error {
	return t.Load(fsys)
}

// Execute executes the template with the given data and writes the result to the given writer.
func (t *TextTemplate) Execute(wr io.Writer, data any) error {
	return t.template.Execute(wr, data)
}
