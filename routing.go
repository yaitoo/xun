package xun

// Routing represents a single route in the router.
type Routing struct {
	Pattern string
	Handle  HandleFunc
	chain   chain

	Options *RoutingOptions
	Viewers map[string]Viewer
}

func (r *Routing) Next(ctx *Context) error {
	return r.chain.Next(r.Handle)(ctx)
}
