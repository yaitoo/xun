package htmx

import (
	"net/http"
)

type JsonViewer struct {
}

func (*JsonViewer) MimeType() string {
	return "application/json"
}

func (j *JsonViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	w.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}
