package xun

import (
	"net/http"
)

// Viewer is the interface that wraps the minimum set of methods required for
// an effective viewer.
type Viewer interface {
	MimeType() string
	Render(w http.ResponseWriter, r *http.Request, data any) error
}
