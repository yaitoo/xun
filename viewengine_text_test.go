package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun/fsnotify"
)

func TestTextViewEngine(t *testing.T) {

	fsys := fstest.MapFS{
		"text/sitemap.xml": {Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>`)},
		"text/robots.txt":  {Data: []byte("User-agent: *")},
		"text/empty.md":    {},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Get("/sitemap.xml", func(c *Context) error {
		return c.View(nil, "text/sitemap.xml")
	})

	app.Get("/robots.txt", func(c *Context) error {
		return c.View(nil, "text/robots.txt")
	})

	app.Get("/empty.md", func(c *Context) error {
		return c.View(nil, "text/empty.md")
	})

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	req.Header.Set("Accept", "application/xml,text/xml,text/plain, */*")
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/sitemap.xml"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/robots.txt", nil)
	req.Header.Set("Accept", "text/plain, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/robots.txt"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/empty.md", nil)
	req.Header.Set("Accept", "text/plain, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Empty(t, buf)

}

func TestWatchOnText(t *testing.T) {
	fsys := fstest.MapFS{
		"text/sitemap.xml": {Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>`), ModTime: time.Now()},
		"text/robots.txt":  {Data: []byte("User-agent: *"), ModTime: time.Now()},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())

	app.Get("/sitemap.xml", func(c *Context) error {
		return c.View(nil, "text/sitemap.xml")
	})

	app.Get("/robots.txt", func(c *Context) error {
		return c.View(nil, "text/robots.txt")
	})

	app.Get("/new.txt", func(c *Context) error {
		return c.View(nil, "text/new.txt")
	})

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/new.txt", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	var content string
	err = json.NewDecoder(resp.Body).Decode(&content)
	require.NoError(t, err)
	require.Empty(t, content)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	req.Header.Set("Accept", "application/xml,text/xml,text/plain, */*")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/sitemap.xml"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/robots.txt", nil)
	req.Header.Set("Accept", "text/plain")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/robots.txt"].Data, buf)

	// fixed data race issue on fstest.MapFile
	app.watcher.Stop()

	fsys["text/sitemap.xml"] = &fstest.MapFile{Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`), ModTime: time.Now()}

	// added
	fsys["text/new.txt"] = &fstest.MapFile{Data: []byte("new content"), ModTime: time.Now()}

	// deleted
	delete(fsys, "text/robots.txt")

	checkInterval := fsnotify.CheckInterval
	fsnotify.CheckInterval = 100 * time.Millisecond
	defer func() {
		fsnotify.CheckInterval = checkInterval
	}()

	go app.watcher.Start()
	time.Sleep(1 * time.Second)

	app.watcher.Stop()

	req, err = http.NewRequest("GET", srv.URL+"/new.txt", nil)
	req.Header.Set("Accept", "text/plain")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/new.txt"].Data, buf)

	req, err = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	req.Header.Set("Accept", "application/xml")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, fsys["text/sitemap.xml"].Data, buf)

	// deleted not be handled, robots.txt still be there
	req, err = http.NewRequest("GET", srv.URL+"/robots.txt", nil)
	req.Header.Set("Accept", "text/plain")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, buf, []byte("User-agent: *"))

}
