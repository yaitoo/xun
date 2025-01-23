package xun

import (
	"fmt"
	"net/http"
)

// StringViewer is a viewer that writes the given data as string to the http.ResponseWriter.
//
// It sets the Content-Type header to "text/plain".
type StringViewer struct {
}

var StringViewerMime = &MimeType{Type: "text", SubType: "plain"}

// MimeType returns the MIME type of the string content.
//
// It returns "text/plain".
func (*StringViewer) MimeType() *MimeType {
	return StringViewerMime
}

// Render renders the given data as string to the http.ResponseWriter.
//
// It sets the Content-Type header to "text/plain; charset=utf-8".
func (*StringViewer) Render(w http.ResponseWriter, r *http.Request, data any) error { // skipcq: RVV-B0012
	if data == nil {
		return nil
	}

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	_, err := fmt.Fprint(w, data)
	return err
}
