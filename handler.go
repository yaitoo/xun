package htmx

// Handler represents an HTTP handler.
type Handler struct {
	Viewers []Viewer

	Pattern string // original string
	Method  string
	Host    string
}
