package htmx

type Handler struct {
	Viewers []Viewer

	Pattern string // original string
	Method  string
	Host    string
}
