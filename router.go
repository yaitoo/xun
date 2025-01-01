package xun

// Router is the interface that wraps the minimum set of methods required for
// an effective router, namely methods for adding routes for different HTTP
// methods, a method for adding middleware, and a method for adding the router
// to the main app.
type Router interface {
	Get(pattern string, h HandleFunc, opts ...RoutingOption)
	Post(pattern string, h HandleFunc, opts ...RoutingOption)
	Put(pattern string, h HandleFunc, opts ...RoutingOption)
	Delete(pattern string, h HandleFunc, opts ...RoutingOption)
	HandleFunc(pattern string, h HandleFunc, opts ...RoutingOption)
	Use(middlewares ...Middleware)
}
