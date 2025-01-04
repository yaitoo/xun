package htmx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestHtmxInterceptor(t *testing.T) {
	m := http.NewServeMux()
	srv := httptest.NewServer(m)
	defer srv.Close()

	app := xun.New(xun.WithMux(m), xun.WithInterceptor(New()))
	defer app.Close()

	app.Get("/redirect", func(c *xun.Context) error {
		c.Redirect("/login", http.StatusFound)
		return nil
	})

	app.Get("/referer", func(c *xun.Context) error {
		return c.View(c.RequestReferer())
	})

	go app.Start()
	defer app.Close()

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { //skipcq: RVV-B0012
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/redirect", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "/login", resp.Header.Get("Location"))

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/redirect", nil)
	require.NoError(t, err)

	req.Header.Set(HxRequest, "true")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "/login", resp.Header.Get(HxRedirect))

	var referer string

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/referer", nil)
	require.NoError(t, err)

	req.Header.Set("Referer", "/home")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&referer)
	require.NoError(t, err)

	require.Equal(t, "/home", referer)

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/referer", nil)
	require.NoError(t, err)

	req.Header.Set(HxRequest, "true")
	req.Header.Set(HxCurrentUrl, "/home")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&referer)
	require.NoError(t, err)

	require.Equal(t, "/home", referer)

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/referer", nil)
	require.NoError(t, err)

	req.Header.Set("Referer", "")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&referer)
	require.NoError(t, err)

	require.Empty(t, referer) // empty referer

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/referer", nil)
	require.NoError(t, err)

	req.Header.Set(HxRequest, "true")
	req.Header.Set(HxCurrentUrl, "")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&referer)
	require.NoError(t, err)

	require.Empty(t, referer)

}
