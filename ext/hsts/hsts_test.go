package hsts

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestHstsMiddleware(t *testing.T) {

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}                           // skipcq: GSC-G402,GO-S1020
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) { // skipcq: RVV-B0012
		if strings.HasPrefix(addr, "abc.com") {
			return net.Dial("tcp", strings.TrimPrefix(addr, "abc.com"))
		}
		return net.Dial("tcp", addr)
	}

	c := http.Client{
		Transport: tr,
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

		app.Use(Enable(1*time.Hour, false, false))

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

		app.Use(Enable(0*time.Hour, false, false))

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

		app.Use(Enable(1*time.Hour, true, false))

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

		app.Use(Enable(1*time.Hour, true, true))

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
		mux := http.NewServeMux()

		srv := &http.Server{ // skipcq: GO-S2112
			Addr:    ":80",
			Handler: mux,
		}
		defer srv.Close()
		go srv.ListenAndServe() // nolint: errcheck

		l := "https://abc.com/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Enable(1*time.Hour, true, true))

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, "http://abc.com/", nil) // skipcq: GO-S1028
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "max-age=3600; includeSubDomains; preload", resp.Header.Get("Strict-Transport-Security"))
	})
}
