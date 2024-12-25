package htmx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingOption(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux))

	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			// check access with metadata
			return next(c)
		}
	})

	app.Get("/admin", func(c *Context) error {

		data := map[string]any{
			"s1":     c.Routing.Options.GetString("s1"),
			"i1":     c.Routing.Options.GetInt("i1"),
			"name":   c.Routing.Options.Get(NavigationName),
			"icon":   c.Routing.Options.Get(NavigationIcon),
			"access": c.Routing.Options.Get(NavigationAccess),
		}

		return c.View(data)
	}, WithMetadata("s1", "v1"),
		WithMetadata("i1", 1),
		WithNavigation("admin", "ha-dash", "admin:view"))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/admin", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	data := make(map[string]any)
	err = json.Unmarshal(buf, &data)
	require.NoError(t, err)

	require.Equal(t, "v1", data["s1"])
	require.EqualValues(t, 1, data["i1"])
	require.Equal(t, "admin", data["name"])
	require.Equal(t, "ha-dash", data["icon"])
	require.Equal(t, "admin:view", data["access"])

}
