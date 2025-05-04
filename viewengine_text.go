package xun

import (
	"io/fs"
	"log/slog"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// TextViewEngine is a view engine that renders text-based templates.
// It watches the file system for changes to text files and updates the corresponding views.
type TextViewEngine struct {
	fsys      fs.FS
	app       *App
	templates map[string]*TextTemplate
}

// Load walks the file system and loads all text-based templates that match the TextViewEngine's pattern.
// It calls the handle method for each matching file to add the template to the app's viewers.
func (ve *TextViewEngine) Load(fsys fs.FS, app *App) {
	if ve.templates == nil {
		ve.templates = map[string]*TextTemplate{}
	}

	ve.fsys = fsys
	ve.app = app

	fs.WalkDir(fsys, "text", func(path string, d fs.DirEntry, err error) error { // nolint: errcheck
		if d != nil && !d.IsDir() {
			if err := ve.loadText(path); err != nil {
				slog.Error("text: load text", slog.String("path", path), slog.Any("err", err))
			}
			return nil
		}

		return nil
	})
}

func (ve *TextViewEngine) loadText(path string) error {
	t, err := ve.loadTemplate(path)
	if err != nil {
		return err
	}

	ve.app.viewers[path] = NewTextViewer(t)

	return err
}

// FileChanged is called when a file in the file system has changed. It checks if the change is a
// file creation event in the "text/" directory, and if so, calls the handle method to update the
// corresponding view in the app.
func (ve *TextViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error { // skipcq: RVV-B0012
	if event.Has(fsnotify.Remove) {
		return nil
	}

	if event.Has(fsnotify.Write) {
		t, ok := ve.templates[event.Name]
		if ok {
			return t.Reload(fsys, app.funcMap)
		}
	} else if event.Has(fsnotify.Create) {

		if strings.HasPrefix(event.Name, "text/") {
			err := ve.loadText(event.Name)
			return err
		}

		return nil
	}

	return nil
}

func (ve *TextViewEngine) loadTemplate(path string) (*TextTemplate, error) {

	t := &TextTemplate{
		name: path,
	}

	if err := t.Load(ve.fsys, ve.app.funcMap); err != nil {
		return nil, err
	}

	ve.templates[path] = t

	return t, nil
}
