package xun

import (
	"net/http"
)

// JsonViewer is a viewer that writes the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
type JsonViewer struct {
}

var jsonViewerMime = &MimeType{Type: "application", SubType: "json"}

// MimeType returns the MIME type of the JSON content.
//
// It returns "application/json".
func (*JsonViewer) MimeType() *MimeType {
	return jsonViewerMime
}

// Render renders the given data as JSON to the http.ResponseWriter.
//
// It sets the Content-Type header to "application/json".
func (*JsonViewer) Render(w http.ResponseWriter, r *http.Request, data any) error { // skipcq: RVV-B0012
	buf := BufPool.Get()
	defer BufPool.Put(buf)

	err := json.NewEncoder(buf).Encode(data)
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	_, err = buf.WriteTo(w)
	return err
}
