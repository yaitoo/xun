package xun

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// TextViewEngine is a view engine that renders text-based templates.
// It watches the file system for changes to text files and updates the corresponding views.
type TextViewEngine struct {
	pattern string
}

// Load walks the file system and loads all text-based templates that match the TextViewEngine's pattern.
// It calls the handle method for each matching file to add the template to the app's viewers.
func (ve *TextViewEngine) Load(fsys fs.FS, app *App) error {

	err := fs.WalkDir(fsys, ".", func(file string, d fs.DirEntry, err error) error {
		if ok, _ := path.Match(ve.pattern, file); !ok {
			return nil
		}

		if err != nil {
			return err
		}

		if !d.IsDir() {
			ve.handle(fsys, app, file)
		}

		return nil
	})

	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}

// FileChanged is called when a file in the file system has changed. It checks if the change is a
// file creation event in the "public/" directory, and if so, calls the handle method to update the
// corresponding view in the app.
func (ve *TextViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	// Nothing should be updated for Write/Remove events.
	if event.Has(fsnotify.Create) && strings.HasPrefix(event.Name, "public/") {
		ve.handle(app, event.Name)
	}

	return nil
}

func (ßve *TextViewEngine) handle(app *App, path string) {

	name := strings.ToLower(path)

	app.viewers["@text/"+name] = &TextViewer{
		template: NewTextTemplate(name, path),
	}
}
