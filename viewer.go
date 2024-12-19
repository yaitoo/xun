package htmx

import (
	"net/http"
)

type Viewer interface {
	MimeType() string
	Render(w http.ResponseWriter, r *http.Request, data any) error
}
