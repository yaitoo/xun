package htmx

import (
	"io"
	"io/fs"
	"strings"
	"text/template"

	"errors"
)

// FuncMap is a map of functions that are available to templates.
var FuncMap template.FuncMap = make(template.FuncMap)

// HtmlTemplate is a template that is loaded from a file system.
type HtmlTemplate struct {
	template *template.Template

	name   string
	path   string
	layout string

	dependencies map[string]struct{}
	dependents   map[string]*HtmlTemplate
}

// NewHtmlTemplate creates a new HtmlTemplate with the given name and path.
func NewHtmlTemplate(name, path string) *HtmlTemplate {
	return &HtmlTemplate{
		name:         name,
		path:         path,
		dependencies: make(map[string]struct{}),
		dependents:   make(map[string]*HtmlTemplate),
	}
}

// Load loads the template from the given file system.
//
// It parses the file, and determines the dependencies of the template.
// The dependencies are stored in the `dependencies` field.
func (t *HtmlTemplate) Load(fsys fs.FS, templates map[string]*HtmlTemplate) error {
	buf, err := fs.ReadFile(fsys, t.path)
	if err != nil {
		return err
	}

	nt := template.New(t.name).Funcs(FuncMap)
	dependencies := make(map[string]struct{})

	defer func() {
		t.template = nt
		t.dependencies = dependencies
	}()

	if len(buf) == 0 {
		return nil
	}

	nt, err = nt.Parse(string(buf))
	if err != nil {
		return err
	}

	for _, it := range nt.Templates() {
		tn := it.Name()
		if strings.EqualFold(tn, t.name) {
			continue
		}

		dependencies[tn] = struct{}{}
	}

	layoutName := ""
	//<!--layout:home-->   xxxxx  \n
	if len(buf) > 11 && string(buf[0:11]) == "<!--layout:" {
		n := len(buf) - 2
		for i := 11; i < n; i++ {
			if buf[i] == '-' && buf[i+1] == '-' && buf[i+2] == '>' {
				layoutName = strings.TrimSpace(string(buf[11:i]))
				break
			}

			if buf[i] == '\n' {
				break
			}
		}

		if layoutName != "" {
			layoutName = "layouts/" + layoutName

			layout, ok := templates[layoutName]
			if ok {
				_, err = nt.AddParseTree(layoutName, layout.template.Tree)
				if err != nil {
					return err
				}

				layout.dependents[t.name] = t

				for tn := range layout.dependencies {
					dependencies[tn] = struct{}{}
				}
			}

			t.layout = layoutName
		} else {
			t.layout = ""
		}
	}

	for tn := range dependencies {
		it, ok := templates[tn]
		if ok {
			_, err = nt.AddParseTree(tn, it.template.Tree)
			if err != nil {
				return err
			}

			it.dependents[t.name] = t
		}
	}

	return nil
}

// Reload reloads the template and all its dependents from the given file system.
//
// It first reloads the current template and then recursively reloads all its dependents.
// If a dependency does not exist, it is removed from the list of dependents.
func (t *HtmlTemplate) Reload(fsys fs.FS, templates map[string]*HtmlTemplate) error {
	err := t.Load(fsys, templates)
	if err != nil {
		return err
	}

	for n, it := range t.dependents {
		err := it.Reload(fsys, templates)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			delete(t.dependents, n)
		}
	}

	return nil
}

// Execute renders the template with the given data and writes the result to the provided writer.
//
// If the template has a layout, it uses the layout to render the data.
// Otherwise, it renders the data using the template itself.
func (t *HtmlTemplate) Execute(wr io.Writer, data any) error {
	if t.layout != "" {
		return t.template.ExecuteTemplate(wr, t.layout, data)
	}
	return t.template.Execute(wr, data)
}
