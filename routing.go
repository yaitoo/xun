package htmx

// Routing represents a single route in the router.
type Routing struct {
	Pattern string
	Handle  HandleFunc

	Options *RoutingOptions
	Viewers map[string]Viewer
}
