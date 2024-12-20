package htmx

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/yaitoo/htmx/fsnotify"
)

type HtmlViewEngine struct {
	fsys fs.FS
	app  *App

	templates map[string]*HtmlTemplate
}

func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) error {
	if ve.templates == nil {
		ve.templates = map[string]*HtmlTemplate{}
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

func (ve *HtmlViewEngine) loadTemplate(path string) (*HtmlTemplate, error) {
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
