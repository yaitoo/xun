package htmx

import (
	"io"
	"io/fs"
	"strings"
	"text/template"

	"errors"
)

type HtmlTemplate struct {
	template *template.Template

	name   string
	path   string
	layout string

	dependencies map[string]struct{}
	dependents   map[string]*HtmlTemplate
}

func NewHtmlTemplate(name, path string) *HtmlTemplate {
	return &HtmlTemplate{
		name:         name,
		path:         path,
		dependencies: make(map[string]struct{}),
		dependents:   make(map[string]*HtmlTemplate),
	}
}

func (t *HtmlTemplate) Load(fsys fs.FS, templates map[string]*HtmlTemplate) error {
	buf, err := fs.ReadFile(fsys, t.path)
	if err != nil {
		return err
	}

	nt := template.New(t.name)
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

func (t *HtmlTemplate) Execute(wr io.Writer, data any) error {
	if t.layout != "" {
		return t.template.ExecuteTemplate(wr, t.layout, data)
	}
	return t.template.Execute(wr, data)
}
