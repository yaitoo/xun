package htmx

import (
	"io/fs"
	"log/slog"
	"net/http"
)

type Option func(*App)

func WithLogger(logger *slog.Logger) Option {
	return func(app *App) {
		app.logger = logger
	}
}

func WithMux(mux *http.ServeMux) Option {
	return func(app *App) {
		app.mux = mux
	}
}

// WithWatch enable hot reload feature, please don't enable it on production. It is not thread-safe.
func WithWatch() Option {
	return func(app *App) {
		app.watch = true
	}
}

func WithFsys(fsys fs.FS) Option {
	return func(app *App) {
		app.fsys = fsys
	}
}

func WithViewer(v Viewer) Option {
	return func(app *App) {
		app.viewer = v
	}
}

func WithViewEngines(ve ...ViewEngine) Option {
	return func(app *App) {
		app.viewEngines = ve
	}
}
