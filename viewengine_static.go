package htmx

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/yaitoo/htmx/fsnotify"
)

type StaticViewEngine struct {
}

func (ve *StaticViewEngine) Load(fsys fs.FS, app *App) error {

	err := fs.WalkDir(fsys, "public", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ve.handle(fsys, app, path)
		}

		return nil
	})

	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}

func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	//Nothing should be updated for Write/Remove events.
	if event.Has(fsnotify.Create) && strings.HasPrefix(event.Name, "public/") {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

	name := strings.ToLower(path)

	if strings.HasSuffix(name, "/index.html") { //remove it, because index.html will be redirected to ./ in http.ServeFileFS
		name = name[:len(name)-10]
	}

	name = strings.TrimPrefix(name, "public/")

	app.HandleFile(name, &FileViewer{
		fsys: fsys,
		path: path,
	})
}
