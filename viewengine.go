package xun

import (
	"io/fs"

	"github.com/yaitoo/xun/fsnotify"
)

// ViewEngine is the interface that wraps the minimum set of methods required for
// an effective view engine, namely methods for loading templates from a file
// system and reloading templates when the file system changes.
type ViewEngine interface {
	Load(fsys fs.FS, app *App) error
	FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error
}
