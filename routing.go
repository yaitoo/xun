package htmx

type Routing struct {
	Pattern string
	Method  string
	Host    string
	Path    string

	Handle HandleFunc

	Options *RoutingOptions
	Viewers map[string]Viewer
}
