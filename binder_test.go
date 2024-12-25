package htmx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBinder(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux))

	app.Start()
	defer app.Close()

}
