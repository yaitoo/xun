package xun

import (
	"embed"
	"errors"
	"io/fs"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// StaticViewEngine is a view engine that serves static files from a file system.
type StaticViewEngine struct {
	isEmbedFsys bool
}

// Load loads all static files from the given file system and registers them with the application.
//
// It scans the "public" directory in the given file system and registers each file
// with the application. It also handles file changes for the "public" directory
// and updates the application accordingly.
func (ve *StaticViewEngine) Load(fsys fs.FS, app *App) error {

	_, ve.isEmbedFsys = fsys.(embed.FS)

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

// FileChanged handles file changes for the given file system and updates the
// application accordingly. It is called by the watcher when a file is changed.
//
// If the file changed is a Create event and the path is in the "public" directory,
// it will be registered with the application.
//
// If the file changed is a Write/Remove event and the path is in the "public"
// directory, nothing will be done.
func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	// Nothing should be updated for Write/Remove events.
	if event.Has(fsnotify.Create) && strings.HasPrefix(event.Name, "public/") {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

	name := strings.ToLower(path)

	if strings.HasSuffix(name, "/index.html") { // remove it, because index.html will be redirected to ./ in http.ServeFileFS
		name = name[:len(name)-10]
	}

	name = strings.TrimPrefix(name, "public/")

	app.HandleFile(name, NewFileViewer(fsys, path, ve.isEmbedFsys))
}
