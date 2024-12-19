package htmx

import (
	"io/fs"
	"strings"

	"github.com/yaitoo/htmx/fsnotify"
)

type StaticViewEngine struct {
}

func (ve *StaticViewEngine) Load(fsys fs.FS, app *App) error {
	return fs.WalkDir(fsys, "public", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ve.handle(fsys, app, path)
		}

		return nil
	})
}

func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	//Nothing should be updated for Write/Remove events.

	if event.Has(fsnotify.Create) {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

	pattern := strings.ToLower(strings.TrimPrefix(path, "public/"))

	if strings.HasSuffix(pattern, "/index.html") {
		pattern = pattern[:len(pattern)-10]
	}

	app.HandleFile(pattern, &FileViewer{
		fsys: fsys,
		path: path,
	})
}
