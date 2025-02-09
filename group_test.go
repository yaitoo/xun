package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

	// NB: pages/admin/index.html is registered by "GET /admin/{$}"". It's precedence is higher than GET /admin/
	// https://go.dev/blog/routing-enhancements.
	// That is why we can pass data into the template by admin.Get("/admin/") here
	admin.Get("/{$}", func(c *Context) error {
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

	req, err := http.NewRequest("GET", srv.URL+"/admin", nil)
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

func TestGroupMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	i := 0

	app := New(WithMux(mux))

	admin := app.Group("/admin")
	admin.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			i++
			c.WriteHeader("X-M1", strconv.Itoa(i))

			return next(c)
		}
	}, func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			i++
			c.WriteHeader("X-M2", strconv.Itoa(i))
			return next(c)
		}
	})

	admin.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			i++
			c.WriteHeader("X-M3", strconv.Itoa(i))
			user := c.Request.Header.Get("X-User")
			if user == "" {
				c.WriteStatus(http.StatusUnauthorized)
				return ErrCancelled
			}

			if user != "yaitoo" {
				c.WriteStatus(http.StatusForbidden)
				return ErrCancelled
			}

			return next(c)
		}
	})

	admin.Get("/", func(c *Context) error {
		return c.View(nil)
	})

	go app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/admin/", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, "1", resp.Header.Get("X-M1"))
	require.Equal(t, "2", resp.Header.Get("X-M2"))
	require.Equal(t, "3", resp.Header.Get("X-M3"))
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	i = 0
	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("X-User", "xun")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	require.Equal(t, "1", resp.Header.Get("X-M1"))
	require.Equal(t, "2", resp.Header.Get("X-M2"))
	require.Equal(t, "3", resp.Header.Get("X-M3"))
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	i = 0
	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("X-User", "yaitoo")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	require.Equal(t, "1", resp.Header.Get("X-M1"))
	require.Equal(t, "2", resp.Header.Get("X-M2"))
	require.Equal(t, "3", resp.Header.Get("X-M3"))
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
