package xun

import (
	"html/template"
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

// WithHandlerViewers sets the Viewer for a route handler.
// If not set, it will use JsonViewer.
func WithHandlerViewers(v ...Viewer) Option {
	return func(app *App) {
		app.handlerViewers = v
	}
}

// WithViewEngines sets the ViewEngines for the App.
// If not set, it will use the default ViewEngines.
func WithViewEngines(ve ...ViewEngine) Option {
	return func(app *App) {
		app.engines = ve
	}
}

// WithInterceptor returns an Option that sets the provided Interceptor
// to the App. This allows customization of the App's behavior by
// intercepting and potentially modifying requests or responses.
//
// Parameters:
//   - i: An Interceptor instance to be set in the App.
//
// Returns:
//   - Option: A function that takes an App pointer and sets its interceptor
//     to the provided Interceptor.
func WithInterceptor(i Interceptor) Option {
	return func(app *App) {
		app.interceptor = i
	}
}

// WithCompressor is an option function that sets the compressors for the application.
// It takes a variadic parameter of Compressor type and assigns it to the app's compressors field.
//
// Parameters:
//
//	c ...Compressor - A variadic list of Compressor instances to be used by the application.
//
// Returns:
//
//	Option - A function that takes an App pointer and sets its compressors field.
func WithCompressor(c ...Compressor) Option {
	return func(app *App) {
		app.compressors = c
	}
}

// WithTemplateFunc adds a custom template function to the application's function map.
func WithTemplateFunc(name string, fn any) Option {
	return func(app *App) {
		app.funcMap[name] = fn
	}
}

// WithTemplateFuncMap adds multiple template functions from the provided map.
func WithTemplateFuncMap(fm template.FuncMap) Option {
	return func(app *App) {
		for name, fn := range fm {
			app.funcMap[name] = fn
		}
	}
}

// WithBuildAssetURL adds a matcher function for identifying assets that need URL processing.
func WithBuildAssetURL(match func(string) bool) Option {
	return func(app *App) {
		app.buildAssetURLs = append(app.buildAssetURLs, match)
	}
}
