package xun

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// HtmlViewEngine is a view engine that loads templates from a file system.
//
// It supports 2 types of templates:
//   - Components: These are templates that are loaded from the "components" directory.
//   - Pages: These are templates that are loaded from the "layouts/views/pages/" directory.
//
// Components are used to build up larger templates, while pages are used to render
// the final HTML that is sent to the client.
type HtmlViewEngine struct {
	fsys fs.FS
	app  *App

	templates map[string]*HtmlTemplate
}

// Load loads all templates from the given file system.
//
// It loads all components, layouts, pages and views from the given file system.
func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) {
	if ve.templates == nil {
		ve.templates = map[string]*HtmlTemplate{}
	}

	ve.fsys = fsys
	ve.app = app

	ve.loadComponents()
	ve.loadLayouts()
	ve.loadPages()
	ve.loadViews()
}

// FileChanged is called when a file has been changed.
//
// It is used to reload templates when they have been changed.
func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error { // skipcq: RVV-B0012

	if event.Has(fsnotify.Remove) || !strings.EqualFold(filepath.Ext(event.Name), ".html") {
		return nil
	}

	name := event.Name[:len(event.Name)-5]

	if event.Has(fsnotify.Write) {
		t, ok := ve.templates[name]
		if ok {
			return t.Reload(fsys, ve.templates, app.funcMap)
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

func (ve *HtmlViewEngine) loadFiles(dir string, process func(path string) error) {
	fs.WalkDir(ve.fsys, dir, func(path string, d fs.DirEntry, _ error) error { // nolint: errcheck
		if !strings.EqualFold(filepath.Ext(path), ".html") {
			return nil
		}
		if err := process(path); err != nil {
			ve.app.logger.Error("xun: load html", slog.String("path", path), slog.Any("err", err))
		}
		return nil
	})
}

func (ve *HtmlViewEngine) loadComponents() {
	ve.loadFiles("components", func(path string) error {
		_, err := ve.loadTemplate(path)
		return err
	})
}

func (ve *HtmlViewEngine) loadTemplate(path string) (*HtmlTemplate, error) {
	name := path[:len(path)-5]

	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
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

func (ve *HtmlViewEngine) loadLayouts() {
	ve.loadFiles("layouts", func(path string) error {
		_, err := ve.loadTemplate(path)
		return err
	})
}

func (ve *HtmlViewEngine) loadPages() {
	ve.loadFiles("pages", func(path string) error { // nolint: errcheck
		return ve.loadPage(path)
	})
}

func (ve *HtmlViewEngine) loadPage(path string) error {
	name := path[6:] // delete prefix  "pages/"

	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
		return err
	}

	// delete file extension ".html"
	ve.templates[path[:len(path)-5]] = t

	if strings.HasSuffix(path, "/index.html") { // remove it, because index.html will be redirected to ./ in http.ServeFileFS
		name = name[:len(name)-10]
	}

	_, _, pattern := splitFile(name)
	pattern = strings.TrimSuffix(pattern, ".html")

	ve.app.HandlePage(pattern, path[6:len(path)-5], &HtmlViewer{
		template: t,
	})

	return nil
}

func (ve *HtmlViewEngine) loadViews() {
	ve.loadFiles("views", func(path string) error {
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
