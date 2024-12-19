package htmx

import (
	"io/fs"

	"github.com/yaitoo/htmx/fsnotify"
)

type ViewEngine interface {
	Load(fsys fs.FS, app *App) error
	FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error
}
