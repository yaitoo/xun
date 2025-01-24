package hsts

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestWriteHeader(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // skipcq: GSC-G402,GO-S1020
	c := http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { // skipcq: RVV-B0012
			return http.ErrUseLastResponse
		},
	}

	tests := []struct {
		name           string
		options        []Option
		expectedHeader string
	}{
		{
			name:           "default_should_work",
			options:        []Option{},
			expectedHeader: "max-age=31536000",
		},
		{
			name:           "max_age_should_work",
			options:        []Option{WithMaxAge(1 * time.Hour)},
			expectedHeader: "max-age=3600",
		},
		{
			name:           "invalid_max_age_should_work",
			options:        []Option{WithMaxAge(0 * time.Hour)},
			expectedHeader: "max-age=31536000",
		},
		{
			name:           "includesubdomains_should_work",
			options:        []Option{WithMaxAge(1 * time.Hour), WithIncludeSubDomains()},
			expectedHeader: "max-age=3600; includeSubDomains",
		},
		{
			name:           "preload_should_work",
			options:        []Option{WithMaxAge(1 * time.Hour), WithPreload()},
			expectedHeader: "max-age=3600; preload",
		},
		{
			name:           "all_should_work",
			options:        []Option{WithMaxAge(1 * time.Hour), WithIncludeSubDomains(), WithPreload()},
			expectedHeader: "max-age=3600; includeSubDomains; preload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			srv := httptest.NewTLSServer(mux)
			defer srv.Close()

			// u, err := url.Parse(srv.URL)
			// require.NoError(t, err)
			// l := "https://" + u.Hostname() + "/"

			app := xun.New(xun.WithMux(mux))
			app.Use(WriteHeader(test.options...))

			app.Get("/", func(c *xun.Context) error {
				return c.View(nil)
			})

			req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
			require.NoError(t, err)
			resp, err := c.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, test.expectedHeader, resp.Header.Get("Strict-Transport-Security")) // default MaxAge is 1 year

		})
	}
}

func TestRedirect(t *testing.T) {

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // skipcq: GSC-G402,GO-S1020
	c := http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { // skipcq: RVV-B0012
			return http.ErrUseLastResponse
		},
	}

	t.Run("http_should_be_redirected", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		l := "https://" + u.Hostname() + "/"
		app := xun.New(xun.WithMux(mux))

		app.Use(Redirect())

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, l, resp.Header.Get("Location"))
		require.Equal(t, "", resp.Header.Get("Strict-Transport-Security"))
	})

	t.Run("https_should_not_be_redirected", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := httptest.NewTLSServer(mux)
		defer srv.Close()
		app := xun.New(xun.WithMux(mux))
		app.Use(Redirect())

		app.Get("/", func(c *xun.Context) error {
			return c.View(nil)
		})

		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "", resp.Header.Get("Location"))
		require.Equal(t, "", resp.Header.Get("Strict-Transport-Security"))
	})

}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name         string
		hostPort     string
		expectedHost string
	}{
		{
			name:         "host_port_should_work",
			hostPort:     "127.0.0.1:8080",
			expectedHost: "127.0.0.1",
		},
		{
			name:         "host_should_work",
			hostPort:     "127.0.0.1",
			expectedHost: "127.0.0.1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host := stripPort(test.hostPort)

			require.Equal(t, test.expectedHost, host)
		})
	}
}
