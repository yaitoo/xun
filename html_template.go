package htmx

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"strings"
	"text/template"
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

	r := bufio.NewReader(bytes.NewReader(buf))
	layoutName, err := r.ReadString('\n')
	if err != nil {
		return err
	}

	layoutName = strings.ReplaceAll(layoutName, " ", "")
	//<!--layout:home-->\n
	if layoutName != "" && strings.HasSuffix(layoutName, "-->\n") && strings.HasPrefix(layoutName, "<!--layout:") {
		layoutName = "layouts/" + layoutName[11:len(layoutName)-4]

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

		for tn := range t.dependencies {
			it, ok := templates[tn]
			if ok {
				_, err = nt.AddParseTree(tn, it.template.Tree)
				if err != nil {
					return err
				}

				it.dependents[t.name] = t
			}
		}

		t.layout = layoutName
	} else {
		t.layout = ""
	}

	return nil
}

func (t *HtmlTemplate) Reload(fsys fs.FS, templates map[string]*HtmlTemplate) error {
	err := t.Load(fsys, templates)
	if err != nil {
		return err
	}

	for _, it := range t.dependents {
		err := it.Reload(fsys, templates)
		if err != nil {
			return err
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
