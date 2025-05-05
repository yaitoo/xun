package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestCache(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := xun.New(xun.WithMux(mux))

	app.Use(New(Match("/starts", "", 1*time.Second),
		Match("", "banner.jpg", 2*time.Second),
		Match("/assets", "app.js", 3*time.Second)))

	app.Get("/starts", func(c *xun.Context) error {
		return c.View(nil)
	})

	app.Get("/banner.jpg", func(c *xun.Context) error {
		return c.View(nil)
	})

	app.Get("/assets/app.js", func(c *xun.Context) error {
		return c.View(nil)
	})

	app.Get("/assets/app.css", func(c *xun.Context) error {
		return c.View(nil)
	})

	app.Get("/logo", func(c *xun.Context) error {
		return c.View(nil)
	})

	resp, err := http.Get(srv.URL + "/starts")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cacheControl := resp.Header.Get("Cache-Control")
	require.Equal(t, "public, max-age=1", cacheControl)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/banner.jpg")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheControl = resp.Header.Get("Cache-Control")
	require.Equal(t, "public, max-age=2", cacheControl)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/assets/app.js")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheControl = resp.Header.Get("Cache-Control")
	require.Equal(t, "public, max-age=3", cacheControl)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/assets/app.css")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheControl = resp.Header.Get("Cache-Control")
	require.Empty(t, cacheControl)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/logo")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheControl = resp.Header.Get("Cache-Control")
	require.Empty(t, cacheControl)
	resp.Body.Close()
}
