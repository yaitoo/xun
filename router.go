package htmx

type Router interface {
	Get(pattern string, h HandleFunc, opts ...RoutingOption)
	Post(pattern string, h HandleFunc, opts ...RoutingOption)
	Put(pattern string, h HandleFunc, opts ...RoutingOption)
	Delete(pattern string, h HandleFunc, opts ...RoutingOption)
	HandleFunc(pattern string, h HandleFunc, opts ...RoutingOption)
	Use(middlewares ...Middleware)
}
