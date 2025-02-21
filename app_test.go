package xun

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

var (
	client http.Client
)

func TestMain(m *testing.M) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) { // skipcq: RVV-B0012
		if strings.HasPrefix(addr, "abc.com") {
			return net.Dial("tcp", strings.TrimPrefix(addr, "abc.com"))
		}
		return net.Dial("tcp", addr)
	}
	client = http.Client{
		Transport: tr,
	}
	os.Exit(m.Run())
}

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

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	err = Json.NewDecoder(resp.Body).Decode(data)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "GET", data.Method)
	require.Equal(t, 1, data.Num)

	req, err = http.NewRequest("POST", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = Json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "POST", data.Method)
	require.Equal(t, 2, data.Num)

	req, err = http.NewRequest("PUT", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = Json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "PUT", data.Method)
	require.Equal(t, 3, data.Num)

	req, err = http.NewRequest("DELETE", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = Json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "DELETE", data.Method)
	require.Equal(t, 4, data.Num)

	req, err = http.NewRequest("GET", srv.URL+"/func", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	err = Json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "HandleFunc", data.Method)
	require.Equal(t, 5, data.Num)

}

func TestStatus(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithHandlerViewers(&JsonViewer{}))

	app.Start()
	defer app.Close()

	app.Get("/400", func(c *Context) error {
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})

	app.Get("/401", func(c *Context) error {
		c.WriteStatus(http.StatusUnauthorized)
		return nil
	})
	app.Get("/403", func(c *Context) error {
		c.WriteStatus(http.StatusForbidden)
		return nil

	})

	app.Get("/404", func(c *Context) error {
		c.WriteStatus(http.StatusNotFound)
		return nil
	})

	app.Get("/500", func(c *Context) error {
		c.WriteStatus(http.StatusInternalServerError)
		return nil
	})

	app.Get("/301", func(c *Context) error {
		c.Redirect("http://127.0.0.1/redirect", http.StatusMovedPermanently)
		return nil
	})

	app.Get("/302", func(c *Context) error {
		c.Redirect("http://127.0.0.1/redirect")
		return nil
	})

	req, err := http.NewRequest("GET", srv.URL+"/400", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/401", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/403", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/404", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/500", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	resp.Body.Close()

	c := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { // skipcq: RVV-B0012
			return http.ErrUseLastResponse
		},
	}

	req, err = http.NewRequest("GET", srv.URL+"/301", nil)
	require.NoError(t, err)
	resp, err = c.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "http://127.0.0.1/redirect", resp.Header.Get("Location"))
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/302", nil)
	require.NoError(t, err)
	resp, err = c.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "http://127.0.0.1/redirect", resp.Header.Get("Location"))
	resp.Body.Close()

}

func TestStaticViewEngine(t *testing.T) {
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
		"public/admin/index.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>admin/index</title>
	</head>
	<body>
		<div hx-get="/admin" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
		"public/assets/skin.css": &fstest.MapFile{
			Data: []byte(`body {
			background: red;
		}`),
		},
		"public/assets/empty.js": &fstest.MapFile{
			Data: []byte(``),
		},
		"public/assets/nil.js": &fstest.MapFile{
			Data: nil,
		},
		// test pattern with host condition
		"public/@abc.com/index.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>abc.com/Index</title>
	</head>
	<body>
		<div hx-get="/" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
		"public/@abc.com/home.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>abc.com/home</title>
	</head>
	<body>
		<div hx-get="/" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
		"public/@abc.com/admin/index.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>	
	<head>
		<meta charset="utf-8">
		<title>abc.com/admin</title>
	</head>
	<body>
		<div hx-get="/" hx-swap="innerHTML"></div>
	</body>
</html>`),
		},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/index.html"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
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

	req, err = http.NewRequest("GET", srv.URL+"/assets/empty.js", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, 0, len(buf))

	req, err = http.NewRequest("GET", srv.URL+"/assets/nil.js", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, 0, len(buf))

	host := strings.ReplaceAll(srv.URL, "127.0.0.1", "abc.com")

	req, err = http.NewRequest("GET", host, nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/@abc.com/index.html"].Data, buf)

	req, err = http.NewRequest("GET", host+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/@abc.com/index.html"].Data, buf)

	req, err = http.NewRequest("GET", host+"/home.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/@abc.com/home.html"].Data, buf)

	req, err = http.NewRequest("GET", host+"/admin", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/@abc.com/admin/index.html"].Data, buf)

	req, err = http.NewRequest("GET", host+"/admin/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["public/@abc.com/admin/index.html"].Data, buf)

}

func TestHtmlViewEngine(t *testing.T) {

	fsys := fstest.MapFS{
		"components/footer.html": {Data: []byte("<div>footer</div>")},
		"components/header.html": {Data: []byte("<div>header</div>")},
		"layouts/main.html": {Data: []byte(`<html><body>{{ block "components/header" . }} {{end}}
<h1>main</h1>
{{ block "content" . }} {{end}}
{{ block "components/footer" . }} {{end}}
</body></html>`)},
		"layouts/admin.html": {Data: []byte(`<html><body>{{ block "components/header" . }} {{end}}
<h1>admin</h1>
{{ block "content" . }} {{end}}
{{ block "components/footer" . }} {{end}}
</body></html>`)},
		"views/user.html": {Data: []byte(`<html><body>{{ block "components/header" . }} {{end}}
<h1>user</h1>
{{ block "components/footer" . }} {{end}}
</body></html>`)},

		"pages/index.html": {Data: []byte(`<!--layout:main-->
{{ define "content" }}<div>index</div>{{ end }}`)},
		"pages/admin/index.html": {Data: []byte(`<!--layout:admin-->
{{ define "content" }}<div>admin/index</div>{{ end }}`)},

		"pages/about.html": {Data: []byte(`<html><body>{{ block "components/header" . }} {{end}}
<h1>about</h1>
{{ block "components/footer" . }} {{end}}
</body></html>`)},

		"pages/@abc.com/index.html": {Data: []byte(`<!--layout:main-->
{{ define "content" }}<div>abc.com/index</div>{{ end }}`)},
		"pages/@abc.com/admin/index.html": {Data: []byte(`<!--layout:admin-->
{{ define "content" }}<div>abc.com/admin/index</div>{{ end }}`)},

		"pages/empty.html": {},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body><div>header</div>
<h1>main</h1>
<div>index</div>
<div>footer</div>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body><div>header</div>
<h1>admin</h1>
<div>admin/index</div>
<div>footer</div>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/about", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body><div>header</div>
<h1>about</h1>
<div>footer</div>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/user", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	host := strings.ReplaceAll(srv.URL, "127.0.0.1", "abc.com")

	req, err = http.NewRequest("GET", host+"/", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body><div>header</div>
<h1>main</h1>
<div>abc.com/index</div>
<div>footer</div>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", host+"/admin/", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body><div>header</div>
<h1>admin</h1>
<div>abc.com/admin/index</div>
<div>footer</div>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/empty", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Empty(t, buf)

}

func TestDataBindOnHtml(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/users.html": {Data: []byte(`<html><body>
<table>
<tbody>
<tr><th>Name</th><th>ID</th></tr>
</tbody>
{{range .Data}}<tr><td>{{.Name}}</td><td>{{.ID}}</td></tr>{{end}}
</tbody>
</table>
</body></html>`)},
		"pages/user/{id}.html": {Data: []byte(`<html><body>
<div>{{ ToUpper .Data.Name}}: {{.Data.ID}}</div>
</body></html>`)},
	}

	type User struct {
		Name string
		ID   int
	}

	users := []User{
		{
			Name: "user1",
			ID:   1,
		},
		{
			Name: "user2",
			ID:   2,
		},
		{
			Name: "user3",
			ID:   3,
		},
	}

	FuncMap["ToUpper"] = strings.ToUpper

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Get("/users", func(c *Context) error {
		return c.View(users)
	})

	app.Get("/user/{id}", func(c *Context) error {
		id := c.Request.PathValue("id")
		for _, user := range users {
			if strconv.Itoa(user.ID) == id {
				return c.View(user)
			}
		}

		c.WriteStatus(http.StatusNotFound)
		return ErrCancelled
	})

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/users", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body>
<table>
<tbody>
<tr><th>Name</th><th>ID</th></tr>
</tbody>
<tr><td>user1</td><td>1</td></tr><tr><td>user2</td><td>2</td></tr><tr><td>user3</td><td>3</td></tr>
</tbody>
</table>
</body></html>`, string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/users", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var list []User
	err = json.Unmarshal(buf, &list)
	require.NoError(t, err)

	require.Equal(t, len(list), 3)
	require.Equal(t, list[0].Name, "user1")
	require.Equal(t, list[0].ID, 1)
	require.Equal(t, list[1].Name, "user2")
	require.Equal(t, list[1].ID, 2)
	require.Equal(t, list[2].Name, "user3")
	require.Equal(t, list[2].ID, 3)

	req, err = http.NewRequest("GET", srv.URL+"/user/4", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/user/4", nil)
	req.Header.Set("Accept", "application/json")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/user/1", nil)
	req.Header.Set("Accept", "text/html, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, `<html><body>
<div>USER1: 1</div>
</body></html>`, string(buf))

}

func TestUnhandledError(t *testing.T) {
	fsys := &fstest.MapFS{
		"public/skin.css": &fstest.MapFile{},
		"pages/user.html": &fstest.MapFile{Data: []byte(`{{.Name }}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	w := bytes.NewBuffer(nil)
	logger := slog.New(slog.NewTextHandler(w, nil))

	app := New(WithMux(mux), WithFsys(fsys), WithLogger(logger))

	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			if c.Request.URL.Path == "/skin.css" {
				return errors.New("file: file is in use by another process")
			} else if c.Request.URL.Path == "/user" {
				return errors.New("file: file is in use by another process")
			}

			return next(c)
		}
	})

	app.Get("/", func(*Context) error {
		return errors.New("internal")
	})

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	// unhandled exception should not be returned to client for security issue. it should be logged on on server side.
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	logId := resp.Header.Get("X-Log-Id")
	require.NotEmpty(t, logId)
	require.Contains(t, w.String(), logId)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/user", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	// unhandled exception should not be returned to client for security issue. it should be logged on on server side.
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	logId = resp.Header.Get("X-Log-Id")
	require.NotEmpty(t, logId)
	require.Contains(t, w.String(), logId)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/skin.css", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	// unhandled exception should not be returned to client for security issue. it should be logged on on server side.
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	logId = resp.Header.Get("X-Log-Id")
	require.NotEmpty(t, logId)
	require.Contains(t, w.String(), logId)
	resp.Body.Close()

}
