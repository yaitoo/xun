package htmx

type Routing struct {
	Pattern string
	Handle  HandleFunc

	Options *RoutingOptions
	Viewers map[string]Viewer
}
