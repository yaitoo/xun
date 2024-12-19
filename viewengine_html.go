package htmx

import (
	"bufio"
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/yaitoo/htmx/fsnotify"
)

type Template struct {
	template *template.Template

	name   string
	path   string
	layout string

	dependencies map[string]struct{}
	dependents   map[string]*Template
}

func NewHtmlTemplate(name, path string) *Template {
	return &Template{
		name:         name,
		path:         path,
		dependencies: make(map[string]struct{}),
		dependents:   make(map[string]*Template),
	}
}

func (t *Template) Load(fsys fs.FS, templates map[string]*Template) error {
	buf, err := fs.ReadFile(fsys, t.path)
	if err != nil {
		return err
	}

	nt, err := template.New(t.name).Parse(string(buf))
	if err != nil {
		return err
	}

	dependencies := make(map[string]struct{})

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

	t.template = nt
	t.dependencies = dependencies

	return nil
}

func (t *Template) Reload(fsys fs.FS, templates map[string]*Template) error {
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

func (t *Template) Execute(wr io.Writer, data any) error {
	if t.layout != "" {
		return t.template.ExecuteTemplate(wr, t.layout, data)
	}
	return t.template.Execute(wr, data)
}

type HtmlViewEngine struct {
	fsys fs.FS
	app  *App

	templates map[string]*Template
}

func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) error {
	if ve.templates == nil {
		ve.templates = map[string]*Template{}
	}

	ve.fsys = fsys
	ve.app = app

	err := ve.loadComponents()
	if err != nil {
		return err
	}

	err = ve.loadLayouts()
	if err != nil {
		return err
	}

	err = ve.loadPages()
	if err != nil {
		return err
	}

	return ve.loadViews()

}

func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {

	if event.Has(fsnotify.Remove) || !strings.EqualFold(filepath.Ext(event.Name), ".html") {
		return nil
	}

	name := event.Name[:len(event.Name)-5]

	if event.Has(fsnotify.Write) {
		t, ok := ve.templates[name]
		if ok {
			return t.Reload(ve.fsys, ve.templates)
		}
	} else if event.Has(fsnotify.Create) {

		if strings.HasPrefix(event.Name, "components/") || strings.HasPrefix(event.Name, "layouts/") {
			_, err := ve.loadTemplate(event.Name)
			return err
		} else if strings.HasPrefix(event.Name, "pages/") {
			return ve.loadPage(event.Name)
		} else if strings.HasPrefix(event.Name, "views/") {
			return ve.loadView(event.Name)
		}

		return nil
	}

	return nil

}

func (ve *HtmlViewEngine) loadComponents() error {

	return fs.WalkDir(ve.fsys, "components", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".html") {
			return nil
		}

		_, err = ve.loadTemplate(path)
		return err
	})

}

func (ve *HtmlViewEngine) loadTemplate(path string) (*Template, error) {
	name := path[:len(path)-5]

	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates); err != nil {
		return nil, err
	}

	for n := range t.dependencies {
		d, ok := ve.templates[n]
		if ok {
			d.dependents[name] = t
		}
	}

	ve.templates[name] = t

	return t, nil
}

func (ve *HtmlViewEngine) loadLayouts() error {
	return fs.WalkDir(ve.fsys, "layouts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {

			_, err = ve.loadTemplate(path)

			return err

		}

		return nil
	})
}

func (ve *HtmlViewEngine) loadPages() error {
	return fs.WalkDir(ve.fsys, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".html") {
			return nil
		}

		return ve.loadPage(path)
	})

}

func (ve *HtmlViewEngine) loadPage(path string) error {
	name := strings.TrimPrefix(path, "pages/")

	_, _, pattern := splitFile(name)
	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates); err != nil {
		return err
	}
	//.html
	ve.templates[path[:len(path)-5]] = t

	if strings.HasSuffix(pattern, "/index.html") {
		pattern = pattern[:len(pattern)-10]
	}

	pattern = strings.TrimSuffix(pattern, ".html")

	ve.app.HandleView(pattern, &HtmlViewer{
		template: t,
	})

	return nil
}

func (ve *HtmlViewEngine) loadViews() error {
	return fs.WalkDir(ve.fsys, "views", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".html") {
			return nil
		}

		return ve.loadView(path)
	})

}

func (ve *HtmlViewEngine) loadView(path string) error {

	t, err := ve.loadTemplate(path)
	if err != nil {
		return err
	}

	ve.app.viewers[path[:len(path)-5]] = &HtmlViewer{
		template: t,
	}

	return nil
}
