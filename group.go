package htmx

import (
	"errors"
	"log/slog"
	"net/http"
)

type group struct {
	prefix      string
	middlewares []Middleware

	app *App
}

func (g *group) Use(middleware ...Middleware) {
	g.middlewares = append(g.middlewares, middleware...)
}

func (g *group) Get(pattern string, hf HandleFunc, opts ...RoutingOption) {
	g.HandleFunc(http.MethodGet+" "+g.prefix+pattern, hf, opts...)
}

func (g *group) Post(pattern string, hf HandleFunc, opts ...RoutingOption) {
	g.HandleFunc(http.MethodPost+" "+g.prefix+pattern, hf, opts...)
}

func (g *group) Put(pattern string, hf HandleFunc, opts ...RoutingOption) {
	g.HandleFunc(http.MethodPut+" "+g.prefix+pattern, hf, opts...)
}

func (g *group) Delete(pattern string, hf HandleFunc, opts ...RoutingOption) {
	g.HandleFunc(http.MethodDelete+" "+g.prefix+pattern, hf, opts...)
}

func (g *group) HandleFunc(pattern string, hf HandleFunc, opts ...RoutingOption) {
	ro := &RoutingOptions{
		viewer: g.app.viewer,
	}
	for _, o := range opts {
		o(ro)
	}

	method, host, path := splitPattern(pattern)

	r, ok := g.app.routes[pattern]

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

		g.app.routes[pattern] = r

		g.app.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			ctx := &Context{
				req:     req,
				rw:      w,
				routing: *r,
				app:     g.app,
			}

			next := r.Handle
			for _, m := range g.middlewares {
				next = m(next)
			}

			err := next(ctx)

			if err != nil {
				if !errors.Is(err, ErrHandleCancelled) {
					logID := nextLogID()
					ctx.Header("X-Log-Id", logID)
					ctx.WriteStatus(http.StatusInternalServerError)
					g.app.logger.Error("htmx: handle", slog.Any("err", err), slog.String("logid", logID))
				}
			}

		})
	}

	if ro.viewer != nil {
		r.Viewers[ro.viewer.MimeType()] = ro.viewer
	}

	viewName := path
	if host != "" {
		viewName = "@" + host + path
	}

	// try to find html viewer
	if v, ok := g.app.viewers[viewName]; ok {
		r.Viewers[v.MimeType()] = v
	}
}
