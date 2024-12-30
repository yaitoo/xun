package htmx

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/yaitoo/htmx/fsnotify"
)

// App is the main struct of the framework.
//
// It is used to register routes, middleware, and view engines.
//
// The application instance is initialized with a new http.ServeMux,
// and a handler that serves files from the current working directory
// is registered.
//
// The application instance is ready to be used with the standard
// http.Server type.
type App struct {
	mu sync.RWMutex

	mux         *http.ServeMux
	middlewares []Middleware
	viewers     map[string]Viewer
	routes      map[string]*Routing
	viewer      Viewer
	viewEngines []ViewEngine
	logger      *slog.Logger
	fsys        fs.FS
	watch       bool
	watcher     *fsnotify.Watcher
}

// New allocates an App instance and loads all view engines.
//
// All view engines are loaded from root directory of given fs.FS.
// If watch is true, it will watch all file changes and reload all view engines if any files are changed.
// If watch is false, it won't watch any file changes.
func New(opts ...Option) *App {
	app := &App{
		routes:  make(map[string]*Routing),
		viewers: make(map[string]Viewer),
		viewer:  &JsonViewer{},
	}

	for _, o := range opts {
		o(app)
	}
	if app.logger == nil {
		app.logger = slog.Default()
	}

	if app.mux == nil {
		app.mux = http.DefaultServeMux
	}

	if app.viewEngines == nil {
		app.viewEngines = []ViewEngine{
			&StaticViewEngine{},
			&HtmlViewEngine{},
		}
	}

	if app.fsys != nil {
		for _, ve := range app.viewEngines {
			err := ve.Load(app.fsys, app)
			if err != nil {
				panic(err)
			}
		}

		if app.watch {
			app.watcher = fsnotify.NewWatcher(app.fsys)
			if err := app.watcher.Add("."); err != nil {
				app.logger.Error("htmx: watcher add", slog.Any("err", err))
			}

			if app.watcher != nil {
				go app.enableHotReload()
			}
		}
	}

	return app
}

type HandleFunc func(c *Context) error

type Middleware func(next HandleFunc) HandleFunc

// Middleware is a function that takes a HandleFunc and returns a HandleFunc.
// Middleware functions are useful for creating reusable pieces of code that can
// be composed together to create complex behavior. For example, a middleware
// function might be used to log each request, or to check if a user is
// authenticated before allowing access to a page.
func (app *App) Group(prefix string) Router {
	return &group{
		prefix: prefix,
		app:    app,
	}
}

func (app *App) enableHotReload() {
	defer app.watcher.Stop()
	go app.watcher.Start()

	for {
		select {
		case event, ok := <-app.watcher.Events:
			if !ok {
				return
			}

			var err error
			for _, ve := range app.viewEngines {
				err = ve.FileChanged(app.fsys, app, event)
				if err != nil {
					app.logger.Error("htmx: on file changed", slog.Any("err", err))
				}
			}

		case err, ok := <-app.watcher.Errors:
			if !ok {
				return
			}

			app.logger.Error("htmx: watcher", slog.Any("err", err))

		}
	}

}

func (app *App) Start() {
	app.mu.Lock()
	defer app.mu.Unlock()

}

func (app *App) Close() {
	app.mu.Lock()
	defer app.mu.Unlock()
}

// Use registers one or more Middleware functions to be executed
// before any route handler. Middleware functions are useful for
// creating reusable pieces of code that can be composed together
// to create complex behavior. For example, a middleware function
// might be used to log each request, or to check if a user is
// authenticated before allowing access to a page.
//
// The order of middleware functions matters. The first middleware
// function that is registered will be executed first, and the last
// middleware function that is registered will be executed last.
//
// Middleware functions are executed in the order they are registered.
func (app *App) Use(middleware ...Middleware) {
	app.middlewares = append(app.middlewares, middleware...)
}

// Get registers a route handler for the given HTTP GET request pattern.
func (app *App) Get(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodGet+" "+pattern, hf, opts...)
}

// Post registers a route handler for the given HTTP POST request pattern.
func (app *App) Post(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodPost+" "+pattern, hf, opts...)
}

// Put registers a route handler for the given HTTP PUT request pattern.
func (app *App) Put(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodPut+" "+pattern, hf, opts...)
}

// Delete registers a route handler for the given HTTP DELETE request pattern.
func (app *App) Delete(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodDelete+" "+pattern, hf, opts...)
}

// HandleFunc registers a route handler for the given HTTP request pattern.
//
// The pattern is expected to be in the format "METHOD PATTERN", where
// METHOD is the HTTP method (e.g. "GET", "POST", etc.) and PATTERN is
// the URL path pattern.
//
// The opts parameter is a list of RoutingOption functions that can be
// used to customize the route. See the RoutingOption type for more
// information.
func (app *App) HandleFunc(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.handleFunc(pattern, hf, opts, app)
}

func (app *App) Next(hf HandleFunc) HandleFunc {
	next := hf
	for i := len(app.middlewares); i > 0; i-- {
		next = app.middlewares[i-1](next)
	}
	return next
}

func (app *App) handleFunc(pattern string, hf HandleFunc, opts []RoutingOption, c chain) {
	ro := &RoutingOptions{
		viewer: app.viewer,
	}
	for _, o := range opts {
		o(ro)
	}

	_, host, path := splitPattern(pattern)

	r, ok := app.routes[pattern]

	if ok {
		r.Options = ro
		r.Handle = hf
		r.chain = c

		if ro.viewer != nil {
			r.Viewers[ro.viewer.MimeType()] = ro.viewer
		}

		return

	}
	r = &Routing{
		Options: ro,
		Pattern: pattern,
		Handle:  hf,
		chain:   c,
		Viewers: make(map[string]Viewer),
	}

	app.routes[pattern] = r

	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{
			req:     req,
			rw:      w,
			Routing: *r,
			app:     app,
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("htmx: handle", slog.Any("err", err), slog.String("logid", logID))
	})

	if ro.viewer != nil {
		r.Viewers[ro.viewer.MimeType()] = ro.viewer
	}

	viewName := path
	if host != "" {
		viewName = "@" + host + "/" + path
	}

	// try to find html viewer
	if v, ok := app.viewers[viewName]; ok {
		r.Viewers[v.MimeType()] = v
	}
}

// HandlePage registers a route handler for a page view.
//
// This function associates a Viewer with a given route pattern
// and registers the route in the application's routing table.
// If a route with the same pattern already exists, it updates
// the existing route with the new Viewer.
func (app *App) HandlePage(pattern string, viewName string, v Viewer) {
	ro := &RoutingOptions{}

	r, ok := app.routes[pattern]
	if ok {
		r.Viewers[v.MimeType()] = v
		return
	}

	ro.viewer = v
	app.viewers[viewName] = v

	hf := func(c *Context) error {
		return v.Render(c.rw, c.req, nil)
	}

	r = &Routing{
		Options: ro,
		Pattern: pattern,
		Handle:  hf,
		chain:   app,
		Viewers: make(map[string]Viewer),
	}

	r.Viewers[v.MimeType()] = v

	app.routes[pattern] = r

	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{
			req:     req,
			rw:      w,
			Routing: *r,
			app:     app,
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("htmx: view", slog.Any("err", err), slog.String("logid", logID))

	})

}

// HandleFile registers a route handler for serving a file.
//
// This function associates a FileViewer with a given file name
// and registers the route in the application's routing table.
// If a route with the same pattern already exists, it returns immediately
// without making any changes.
func (app *App) HandleFile(name string, v *FileViewer) {
	ro := &RoutingOptions{}

	_, _, pat := splitFile(name)

	r, ok := app.routes[pat]

	if ok {
		return
	}

	ro.viewer = v
	app.viewers[name] = v

	hf := func(c *Context) error {
		return v.Render(c.rw, c.req, nil)
	}

	r = &Routing{
		Options: ro,
		Pattern: pat,
		Handle:  hf,
		chain:   app,
		Viewers: make(map[string]Viewer),
	}

	app.routes[pat] = r

	r.Viewers[v.MimeType()] = v

	app.mux.HandleFunc(pat, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{
			req:     req,
			rw:      w,
			Routing: *r,
			app:     app,
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("htmx: file", slog.Any("err", err), slog.String("logid", logID))
	})
}
