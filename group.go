package htmx

import (
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
	g.app.handleFunc(pattern, hf, opts, g.middlewares)
}
