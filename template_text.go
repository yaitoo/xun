package xun

import (
	"io"
	"io/fs"
	"text/template"
)

// TextFuncMap is a template.FuncMap that contains custom functions for use in text templates.
var TextFuncMap template.FuncMap = make(template.FuncMap)

// NewTextTemplate creates a new TextTemplate with the given name and path.
func NewTextTemplate(name, path string) *TextTemplate {
	return &TextTemplate{
		name: name,
		path: path,
	}
}

// TextTemplate represents a text template that can be loaded from a file system and executed with data.
type TextTemplate struct {
	template *template.Template

	name string
	path string
}

// Load loads the template from the given file system.
func (t *TextTemplate) Load(fsys fs.FS, templates map[string]*TextTemplate) error {
	buf, err := fs.ReadFile(fsys, t.path)
	if err != nil {
		return err
	}

	nt := template.New(t.name).Funcs(TextFuncMap)

	defer func() {
		t.template = nt
	}()

	if len(buf) == 0 {
		return nil
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
