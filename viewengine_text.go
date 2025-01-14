package xun

import (
	"errors"
	"io/fs"
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
func (ve *TextViewEngine) Load(fsys fs.FS, app *App) error {
	if ve.templates == nil {
		ve.templates = map[string]*TextTemplate{}
	}

	ve.fsys = fsys
	ve.app = app

	err := fs.WalkDir(fsys, "text", func(path string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		if !d.IsDir() {
			return ve.loadText(path)

		}

		return nil
	})

	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}

func (ve *TextViewEngine) loadText(path string) error {
	t, err := ve.loadTemplate(path)
	if err != nil {
		return err
	}

	ve.app.viewers[path] = &TextViewer{
		template: t,
	}
	return err
}

// FileChanged is called when a file in the file system has changed. It checks if the change is a
// file creation event in the "text/" directory, and if so, calls the handle method to update the
// corresponding view in the app.
func (ve *TextViewEngine) FileChanged(fsys fs.FS, _ *App, event fsnotify.Event) error { // skipcq: RVV-B0012
	if event.Has(fsnotify.Remove) {
		return nil
	}

	if event.Has(fsnotify.Write) {
		t, ok := ve.templates[event.Name]
		if ok {
			return t.Reload(fsys)
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

	if err := t.Load(ve.fsys); err != nil {
		return nil, err
	}

	ve.templates[path] = t

	return t, nil
}
