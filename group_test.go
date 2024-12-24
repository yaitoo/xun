package htmx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {

	fsys := &fstest.MapFS{
		"pages/admin/index.html": &fstest.MapFile{Data: []byte(`{{.}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Start()
	defer app.Close()

	admin := app.Group("/admin")

	admin.Get("/", func(c *Context) error {
		return c.View("GET", "admin/index")
	})

	admin.Post("/", func(c *Context) error {
		return c.View("POST", "admin/index")
	})

	admin.Put("/", func(c *Context) error {
		return c.View("PUT", "admin/index")
	})

	admin.Delete("/", func(c *Context) error {
		return c.View("DELETE", "admin/index")
	})

	req, err := http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `GET`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "\"GET\"\n", string(buf))

	req, err = http.NewRequest("POST", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `POST`, string(buf))

	req, err = http.NewRequest("POST", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "\"POST\"\n", string(buf))

	req, err = http.NewRequest("PUT", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `PUT`, string(buf))

	req, err = http.NewRequest("PUT", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "\"PUT\"\n", string(buf))

	req, err = http.NewRequest("DELETE", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `DELETE`, string(buf))

	req, err = http.NewRequest("DELETE", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "\"DELETE\"\n", string(buf))

}
