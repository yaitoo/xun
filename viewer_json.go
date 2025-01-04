package xun

import (
	"net/http"
)

// JsonViewer is a viewer that writes the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
type JsonViewer struct {
}

// MimeType returns the MIME type of the JSON content.
//
// It returns "application/json".
func (*JsonViewer) MimeType() string {
	return "application/json"
}

// Render renders the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
func (*JsonViewer) Render(w http.ResponseWriter, r *http.Request, data any) error {
	w.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}
