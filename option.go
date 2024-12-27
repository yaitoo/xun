package htmx

import (
	"io/fs"
	"log/slog"
	"net/http"
)

// Option is a function that takes a pointer to an App and modifies it.
// It is used to configure an App when calling the New function.
type Option func(*App)

// WithLogger sets the logger for the App. If not set, it will use slog.Default()
func WithLogger(logger *slog.Logger) Option {
	return func(app *App) {
		app.logger = logger
	}
}

// WithMux sets the http.ServeMux for the App. If not set, it will use http.DefaultServeMux.
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

// WithFsys sets the fs.FS for the App. If not set, Page Router is disabled.
func WithFsys(fsys fs.FS) Option {
	return func(app *App) {
		app.fsys = fsys
	}
}

// WithViewer sets the default Viewer for the App.
// If not set, it will use JsonViewer.
func WithViewer(v Viewer) Option {
	return func(app *App) {
		app.viewer = v
	}
}

// WithViewEngines sets the ViewEngines for the App.
// If not set, it will use the default ViewEngines.
func WithViewEngines(ve ...ViewEngine) Option {
	return func(app *App) {
		app.viewEngines = ve
	}
}
