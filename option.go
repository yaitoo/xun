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

// WithContentDir overrides the directory in which .md files are discovered.
// Default is "content". Pass "" to disable Markdown content loading entirely.
//
// The default HtmlViewEngine will scan this directory at Load time and on
// file-change events for .md and .html files.
func WithContentDir(dir string) Option {
	return func(app *App) {
		for _, ve := range app.engines {
			if hve, ok := ve.(*HtmlViewEngine); ok {
				hve.contentDir = dir
			}
		}
	}
}

// WithContentRenderer replaces the default Markdown rendering with a custom
// function. Use this to plug in a fully-configured goldmark instance,
// add syntax highlighting, AST-level class injection, or any other transform.
//
// The function receives the raw file bytes and the file path; it must return
// the rendered template.HTML.
func WithContentRenderer(
	render func(content []byte, path string) (template.HTML, error),
) Option {
	return func(app *App) {
		for _, ve := range app.engines {
			if hve, ok := ve.(*HtmlViewEngine); ok {
				hve.renderFn = render
			}
		}
	}
}

// WithContentMeta replaces the default metadata extraction with a custom
// function. The default derives Title from the first # H1 and Description
// from the first blockquote/paragraph, but you may want to read fields from
// a sidecar file, an embedded database, or external metadata.
//
// The function receives the file path, raw bytes, and fs.FileInfo; it must
// return a fully populated ContentView (Body is filled later by the renderer).
func WithContentMeta(
	fn func(path string, content []byte, fi fs.FileInfo) ContentView,
) Option {
	return func(app *App) {
		for _, ve := range app.engines {
			if hve, ok := ve.(*HtmlViewEngine); ok {
				hve.metaExtractor = fn
			}
		}
	}
}
