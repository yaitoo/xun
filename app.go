package htmx

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/yaitoo/htmx/fsnotify"
)

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

// New returns a new instance of the App struct.
//
// The application instance is initialized with a
// new http.ServeMux, and a handler that serves files
// from the current working directory is registered.
//
// The application instance is ready to be used with
// the standard http.Server type.
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
				app.logger.Error("htmx: viewengine load", slog.Any("err", err))
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

func (app *App) Use(middleware ...Middleware) {
	app.middlewares = append(app.middlewares, middleware...)
}

func (app *App) Get(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodGet+" "+pattern, hf, opts...)
}

func (app *App) Post(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodPost+" "+pattern, hf, opts...)
}

func (app *App) Put(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodPut+" "+pattern, hf, opts...)
}

func (app *App) Delete(pattern string, hf HandleFunc, opts ...RoutingOption) {
	app.HandleFunc(http.MethodDelete+" "+pattern, hf, opts...)
}

func (app *App) HandleFunc(pattern string, hf HandleFunc, opts ...RoutingOption) {
	ro := &RoutingOptions{
		viewer: app.viewer,
	}
	for _, o := range opts {
		o(ro)
	}

	method, host, path := splitPattern(pattern)

	r, ok := app.routes[pattern]

	if ok {
		r.Options = ro
		r.Handle = hf

	} else {
		r = &Routing{
			Options: ro,
			Pattern: pattern,
			Method:  method,
			Host:    host,
			Path:    path,
			Handle:  hf,
			Viewers: make(map[string]Viewer),
		}

		app.routes[pattern] = r

		app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			ctx := &Context{
				req:     req,
				rw:      w,
				routing: *r,
				app:     app,
			}

			next := r.Handle
			for _, m := range app.middlewares {
				next = m(next)
			}

			err := next(ctx)

			if err == nil || errors.Is(err, ErrCancelled) {
				return
			}

			logID := nextLogID()
			ctx.Header("X-Log-Id", logID)
			ctx.WriteStatus(http.StatusInternalServerError)
			app.logger.Error("htmx: handle", slog.Any("err", err), slog.String("logid", logID))
		})
	}

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

func (app *App) HandlePage(pattern string, v Viewer) {
	ro := &RoutingOptions{}

	r, ok := app.routes[pattern]
	if ok {
		r.Viewers[v.MimeType()] = v
		return
	}

	_, host, path := splitPattern(pattern)

	ro.viewer = v
	if host == "" {
		app.viewers[path] = v
	} else {
		app.viewers["@"+host+"/"+path] = v
	}

	hf := func(c *Context) error {
		return v.Render(c.rw, c.req, nil)
	}

	r = &Routing{
		Options: ro,
		Pattern: pattern,
		Host:    host,
		Path:    path,
		Handle:  hf,
		Viewers: make(map[string]Viewer),
	}

	r.Viewers[v.MimeType()] = v

	app.routes[pattern] = r

	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{
			req:     req,
			rw:      w,
			routing: *r,
			app:     app,
		}

		next := r.Handle
		for _, m := range app.middlewares {
			next = m(next)
		}

		err := next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.Header("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("htmx: view", slog.Any("err", err), slog.String("logid", logID))

	})

}

func (app *App) HandleFile(name string, v *FileViewer) {
	ro := &RoutingOptions{}

	host, path, pat := splitFile(name)

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
		Host:    host,
		Path:    path,
		Handle:  hf,
		Viewers: make(map[string]Viewer),
	}

	app.routes[pat] = r

	app.mux.HandleFunc(pat, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{
			req:     req,
			rw:      w,
			routing: *r,
			app:     app,
		}

		next := r.Handle
		for _, m := range app.middlewares {
			next = m(next)
		}

		err := next(ctx)

		if err == nil || errors.Is(err, ErrCancelled) {
			return
		}

		logID := nextLogID()
		ctx.Header("X-Log-Id", logID)
		ctx.WriteStatus(http.StatusInternalServerError)
		app.logger.Error("htmx: file", slog.Any("err", err), slog.String("logid", logID))
	})
}
