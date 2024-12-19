package htmx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestJsonViewer(t *testing.T) {

	m := http.NewServeMux()
	srv := httptest.NewServer(m)
	defer srv.Close()

	app := New(WithMux(m))
	defer app.Close()

	app.Get("/", func(c *Context) error {
		return c.View(map[string]any{"method": "GET", "num": 1})
	})

	app.Post("/", func(c *Context) error {
		return c.View(map[string]any{"method": "POST", "num": 2})
	})

	app.Put("/", func(c *Context) error {
		return c.View(map[string]any{"method": "PUT", "num": 3})
	})

	app.Delete("/", func(c *Context) error {
		return c.View(map[string]any{"method": "DELETE", "num": 4})
	})

	app.HandleFunc("GET /func", func(c *Context) error {
		return c.View(map[string]any{"method": "HandleFunc", "num": 5})
	})

	go app.Start()

	data := &struct {
		Method string `json:"method"`
		Num    int    `json:"num"`
	}{}

	client := http.Client{}

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	err = json.NewDecoder(resp.Body).Decode(data)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "GET", data.Method)
	require.Equal(t, 1, data.Num)

	req, err = http.NewRequest("POST", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "POST", data.Method)
	require.Equal(t, 2, data.Num)

	req, err = http.NewRequest("PUT", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "PUT", data.Method)
	require.Equal(t, 3, data.Num)

	req, err = http.NewRequest("DELETE", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "DELETE", data.Method)
	require.Equal(t, 4, data.Num)

	req, err = http.NewRequest("GET", srv.URL+"/func", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "HandleFunc", data.Method)
	require.Equal(t, 5, data.Num)

}

func TestStaticViewer(t *testing.T) {
	fsys := fstest.MapFS{
		"public/index.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>Index</title>
	</head>
	<body>
		<div hx-get="/" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
		"public/home.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>Home</title>
	</head>
	<body>
		<div hx-get="/home" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
		"public/assets/skin.css": &fstest.MapFile{
			Data: []byte(`body {
			background: red;
		}`),
		},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Start()
	defer app.Close()

	client := http.Client{}

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/index.html"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/home.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/home.html"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/assets/skin.css", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/assets/skin.css"].Data, buf)

}

func TestApp(t *testing.T) {
	app := New(WithMux(http.NewServeMux()),
		WithFsys(os.DirFS(".")))

	app.Get("/hello", func(c *Context) error {
		//c.View(map[string]string{"name": "World"})

		return nil
	})

	admin := app.Group("/admin")

	admin.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			if c.routing.Options.String(NavigationAccess) != "admin:*" {
				c.WriteStatus(http.StatusForbidden)
				return ErrHandleCancelled
			}

			return next(c)
		}

	})

	admin.Get("/", func(c *Context) error {
		return c.View(nil)

	}, WithNavigation("admin", "fa fa-home", "admin:*"))

	admin.Post("/form", func(c *Context) error {
		data, err := BindJSON[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrHandleCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})

	admin.Get("/search", func(c *Context) error {
		data, err := BindQuery[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrHandleCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})

	admin.Post("/form", func(c *Context) error {
		data, err := BindForm[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrHandleCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})
}

type TestData struct {
}
