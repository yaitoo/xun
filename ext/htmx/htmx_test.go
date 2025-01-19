package htmx

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestHtmxWriteHeader(t *testing.T) {

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := xun.New(xun.WithMux(mux))

	app.Get("/string", func(c *xun.Context) error {
		WriteHeader(c, HxTrigger, "string")
		return c.View(nil)
	})

	app.Get("/int", func(c *xun.Context) error {
		WriteHeader(c, HxTrigger, 100)
		return c.View(nil)
	})

	app.Get("/header", func(c *xun.Context) error {
		WriteHeader(c, HxTrigger, HxHeader[string]{"name": "message"})
		return c.View(nil)
	})

	client := &http.Client{}
	var value string

	req, err := http.NewRequest("GET", srv.URL+"/string", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "string", resp.Header.Get(HxTrigger))

	req, err = http.NewRequest("GET", srv.URL+"/int", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	value = resp.Header.Get(HxTrigger)
	i, err := strconv.Atoi(value)
	require.NoError(t, err)
	require.Equal(t, 100, i)

	var header HxHeader[string]
	req, err = http.NewRequest("GET", srv.URL+"/header", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	value = resp.Header.Get(HxTrigger)
	err = json.Unmarshal([]byte(value), &header)
	require.NoError(t, err)
	require.Equal(t, "message", header["name"])

}
