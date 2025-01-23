package hsts

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestHstsMiddleware(t *testing.T) {
	c := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { // skipcq: RVV-B0012
			return http.ErrUseLastResponse
		},
	}

	t.Run("max_age_should_work", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		l := "https://" + u.Hostname() + "/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Enable(WithMaxAge(1*time.Hour), WithDomains(false), WithPreload(false)))

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "max-age=3600", resp.Header.Get("Strict-Transport-Security")) // default MaxAge is 1 year
	})

	t.Run("invalid_max_age_should_work", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		l := "https://" + u.Hostname() + "/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Enable(WithDomains(false), WithPreload(false)))

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "max-age=31536000", resp.Header.Get("Strict-Transport-Security"))
	})

	t.Run("max_age_includesubdomains_should_work", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		l := "https://" + u.Hostname() + "/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Enable(WithMaxAge(1*time.Hour), WithDomains(true), WithPreload(false)))

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "max-age=3600; includeSubDomains", resp.Header.Get("Strict-Transport-Security"))
	})

	t.Run("max_age_includesubdomains_preload_should_work", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		l := "https://" + u.Hostname() + "/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Enable(WithMaxAge(1*time.Hour), WithDomains(true), WithPreload(true)))

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "max-age=3600; includeSubDomains; preload", resp.Header.Get("Strict-Transport-Security"))
	})

	t.Run("without_port_should_work", func(t *testing.T) {
		host := stripPort("abc.com")

		require.Equal(t, "abc.com", host)
	})
}
