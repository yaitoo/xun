package htmx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	app.Get("/", func(c *Context) error {
		data := map[string]any{
			"s1":     c.Routing.Options.GetString("s1"),
			"i1":     c.Routing.Options.GetInt("i1"),
			"name":   c.Routing.Options.Get(NavigationName),
			"icon":   c.Routing.Options.Get(NavigationIcon),
			"access": c.Routing.Options.Get(NavigationAccess),
		}

		return c.View(data)

	}, WithMetadata("s1", "v1"),
		WithMetadata("i1", 1))

	app.Get("/admin", func(c *Context) error {

		data := map[string]any{
			"s1":     c.Routing.Options.GetString("s1"),
			"i1":     c.Routing.Options.GetInt("i1"),
			"name":   c.Routing.Options.Get(NavigationName),
			"icon":   c.Routing.Options.Get(NavigationIcon),
			"access": c.Routing.Options.Get(NavigationAccess),
		}

		return c.View(data)
	},
		WithNavigation("admin", "ha-dash", "admin:view"))

	app.Get("/invalid", func(c *Context) error {
		data := map[string]any{
			"s1":     c.Routing.Options.GetString("s1"),
			"i1":     c.Routing.Options.GetInt("i1"),
			"name":   c.Routing.Options.Get(NavigationName),
			"icon":   c.Routing.Options.Get(NavigationIcon),
			"access": c.Routing.Options.Get(NavigationAccess),
		}

		return c.View(data)
	}, WithMetadata("s1", time.Now()), WithMetadata("i1", "v100"),
		WithNavigation("admin", "ha-dash", "admin:view"))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	d1 := make(map[string]any)
	err = json.Unmarshal(buf, &d1)
	require.NoError(t, err)

	require.Equal(t, "v1", d1["s1"])
	require.EqualValues(t, 1, d1["i1"])
	require.Nil(t, d1["name"])
	require.Nil(t, d1["icon"])
	require.Nil(t, d1["access"])

	req, err = http.NewRequest("GET", srv.URL+"/admin", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	d2 := make(map[string]any)
	err = json.Unmarshal(buf, &d2)
	require.NoError(t, err)

	require.Equal(t, "", d2["s1"])
	require.EqualValues(t, 0, d2["i1"])
	require.Equal(t, "admin", d2["name"])
	require.Equal(t, "ha-dash", d2["icon"])
	require.Equal(t, "admin:view", d2["access"])

	req, err = http.NewRequest("GET", srv.URL+"/invalid", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	d3 := make(map[string]any)
	err = json.Unmarshal(buf, &d3)
	require.NoError(t, err)

	require.Equal(t, "", d3["s1"])
	require.EqualValues(t, 0, d3["i1"])
	require.Nil(t, d3["name"])
	require.Nil(t, d3["icon"])
	require.Nil(t, d3["access"])

}
