package xun

import (
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/yaitoo/xun/fsnotify"
)

// HandleFunc defines a function type that takes a Context pointer as an argument
// and returns an error. It is used to handle requests within the application.
type HandleFunc func(c *Context) error

// Middleware is a function type that takes a HandleFunc as an argument and returns a HandleFunc.
// It is used to wrap or decorate an existing HandleFunc with additional functionality.
type Middleware func(next HandleFunc) HandleFunc

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

	mux            *http.ServeMux
	middlewares    []Middleware
	viewers        map[string]Viewer
	routes         map[string]*Routing
	handlerViewers []Viewer
	engines        []ViewEngine
	logger         *slog.Logger
	fsys           fs.FS
	watch          bool
	watcher        *fsnotify.Watcher
	interceptor    Interceptor
	compressors    []Compressor

	funcMap        template.FuncMap
	buildAssetURLs []func(string) bool
	AssetURLs      map[string]string
}

// New allocates an App instance and loads all view engines.
//
// All view engines are loaded from root directory of given fs.FS.
// If watch is true, it will watch all file changes and reload all view engines if any files are changed.
// If watch is false, it won't watch any file changes.
func New(opts ...Option) *App {
	app := &App{
		routes:         make(map[string]*Routing),
		viewers:        make(map[string]Viewer),
		handlerViewers: []Viewer{&JsonViewer{}},
		funcMap:        builtins,
		AssetURLs:      make(map[string]string),
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

	if app.engines == nil {
		app.engines = []ViewEngine{
			&StaticViewEngine{},
			&HtmlViewEngine{},
			&TextViewEngine{},
		}
	}

	if app.fsys != nil {
		app.funcMap["asset"] = app.getAssetUrl

		for _, ve := range app.engines {
			ve.Load(app.fsys, app)
		}

		if app.watch {
			app.watcher = fsnotify.NewWatcher(app.fsys)
			if err := app.watcher.Add("."); err != nil {
				app.logger.Error("xun: watcher add", slog.Any("err", err))
			} else {
				go app.enableHotReload()
			}
		}
	}

	return app
}

func (app *App) getAssetUrl(pattern string) string {
	// lock-free, because AssetURLs is initialized in New() in production
	if _, ok := app.AssetURLs[pattern]; ok {
		return app.AssetURLs[pattern]
	}

	return pattern
}

// Group creates a new router group with the specified prefix.
// It returns a Router interface that can be used to define routes
// within the group.
func (app *App) Group(prefix string) Router {
	return &group{
		prefix: prefix,
		app:    app,
	}
}

// Start initializes and starts the application by locking the mutex,
// iterating through the routes, and logging the pattern and viewers
// for each route. It ensures thread safety by using a mutex lock.
func (app *App) Start() {
	app.mu.Lock()
	defer app.mu.Unlock()

	for _, r := range app.routes {
		keys := make([]string, 0, len(r.Viewers))
		for _, v := range r.Viewers {
			keys = append(keys, v.MimeType().String())
		}

		app.logger.Info(r.Pattern, slog.String("viewer", strings.Join(keys, ",")))
	}

}

// Close safely locks the App instance, ensuring that no other
// goroutines can access it until the lock is released. This method
// should be called when the App instance is no longer needed to
// prevent any further operations on it.
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

// Next applies the middlewares in the app to the given HandleFunc in reverse order.
// It returns the final HandleFunc after all middlewares have been applied.
func (app *App) Next(hf HandleFunc) HandleFunc {
	next := hf
	for i := len(app.middlewares); i > 0; i-- {
		next = app.middlewares[i-1](next)
	}
	return next
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

	ro.viewers = []Viewer{v}
	app.viewers[name] = v

	hf := func(c *Context) error {
		return v.Render(c, nil)
	}

	r = &Routing{
		Options: ro,
		Pattern: pat,
		Handle:  hf,
		chain:   app,
	}

	app.routes[pat] = r

	r.Viewers = append(r.Viewers, v)

	app.mux.HandleFunc(pat, func(w http.ResponseWriter, req *http.Request) {
		rw := app.createWriter(req, w)
		defer rw.Close()

		ctx := &Context{
			Request:  req,
			Response: rw,
			Routing:  *r,
			App:      app,
			TempData: make(map[string]any),
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("xun: file", slog.Any("err", err), slog.String("logid", logID))
	})
}

// HandlePage registers a route handler for a page view.
//
// This function associates a Viewer with a given route pattern
// and registers the route in the application's routing table.
// If a route with the same pattern already exists, it updates
// the existing route with the new Viewer.
func (app *App) HandlePage(pattern string, viewName string, v Viewer) {
	ro := &RoutingOptions{
		viewers: []Viewer{v},
	}

	r, ok := app.routes[pattern]
	if ok {
		r.Viewers = append(r.Viewers, v)
		return
	}

	app.viewers[viewName] = v

	hf := func(c *Context) error {
		return v.Render(c, nil)
	}

	r = &Routing{
		Options: ro,
		Pattern: pattern,
		Handle:  hf,
		chain:   app,
	}

	r.Viewers = append(r.Viewers, v)

	app.routes[pattern] = r

	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		rw := app.createWriter(req, w)
		defer rw.Close()

		ctx := &Context{
			Request:  req,
			Response: rw,
			Routing:  *r,
			App:      app,
			TempData: make(map[string]any),
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("xun: view", slog.Any("err", err), slog.String("logid", logID))

	})

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
	app.createHandler(pattern, hf, opts, app)
}

// createWriter creates a ResponseWriter that supports compression based on the
// "Accept-Encoding" header in the HTTP request. If the header contains a
// supported encoding or a wildcard "*", it returns a compressed ResponseWriter.
// Otherwise, it returns a standard ResponseWriter.
func (app *App) createWriter(req *http.Request, w http.ResponseWriter) ResponseWriter {
	acceptEncoding := req.Header.Get("Accept-Encoding")

	stars := strings.ContainsAny(acceptEncoding, "*")

	for _, compressor := range app.compressors {
		if stars || strings.Contains(acceptEncoding, compressor.AcceptEncoding()) {
			return compressor.New(w)
		}
	}
	return &stdResponseWriter{ResponseWriter: w}
}

// createHandler registers a new route with the given pattern, handler function, routing options, and middleware chain.
// It updates the route if it already exists or creates a new one if it doesn't.
// The function also sets up the HTTP handler for the route and manages the viewers for different MIME types.
func (app *App) createHandler(pattern string, hf HandleFunc, opts []RoutingOption, c chain) {
	ro := &RoutingOptions{
		viewers: app.handlerViewers,
	}
	for _, o := range opts {
		o(ro)
	}

	r, ok := app.routes[pattern]

	if ok {
		// overwrite existing page route
		r.Options = ro
		r.Handle = hf
		r.chain = c

		if len(ro.viewers) > 0 {
			// append current handler's viewer to existing viewers
			r.Viewers = append(r.Viewers, ro.viewers...)
		}

		return

	}

	r = &Routing{
		Options: ro,
		Pattern: pattern,
		Handle:  hf,
		chain:   c,
	}

	if len(ro.viewers) > 0 {
		// append current handler's viewer to first viewer
		r.Viewers = append(r.Viewers, ro.viewers...)
	}

	app.routes[pattern] = r

	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		rw := app.createWriter(req, w)
		defer rw.Close()

		ctx := &Context{
			Request:  req,
			Response: rw,
			Routing:  *r,
			App:      app,
			TempData: make(map[string]any),
		}

		err := r.Next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		if errors.Is(err, ErrViewNotFound) {
			ctx.WriteStatus(http.StatusNotFound)
			ctx.Response.Write([]byte("View Not Found")) // nolint: errcheck
			return
		}

		logID := nextLogID()
		ctx.WriteHeader("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("xun: handle", slog.Any("err", err), slog.String("logid", logID))
	})

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
			for _, ve := range app.engines {
				err = ve.FileChanged(app.fsys, app, event)
				if err != nil {
					app.logger.Error("xun: on file changed", slog.Any("err", err))
				}
			}

		case err, ok := <-app.watcher.Errors:
			if !ok {
				return
			}
			app.logger.Error("xun: watcher", slog.Any("err", err))
		}
	}

}
