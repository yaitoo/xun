package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	i := 0

	app := New(WithMux(mux))
	app.Use(func(next HandleFunc) HandleFunc {
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

	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			i++
			c.WriteHeader("X-M3", strconv.Itoa(i))
			user := c.Request().Header.Get("X-User")
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

	app.Get("/", func(c *Context) error {
		return c.View(nil)
	})

	go app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
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
	req, err = http.NewRequest("GET", srv.URL+"/", nil)
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
	req, err = http.NewRequest("GET", srv.URL+"/", nil)
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
