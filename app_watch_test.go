package xun

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun/fsnotify"
)

func TestWatchOnStatic(t *testing.T) {
	fsys := fstest.MapFS{
		"public/home.html":  {Data: []byte("home"), Mode: os.ModePerm, ModTime: time.Now()},
		"public/admin.html": {Data: []byte("admin"), Mode: os.ModePerm, ModTime: time.Now()},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch(), WithViewEngines(&StaticViewEngine{}))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/home.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "home", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "admin", string(buf))

	// fixed data race issue on fstest.MapFile
	app.watcher.Stop()
	fsys["public/index.html"] = &fstest.MapFile{Data: []byte("index added"), ModTime: time.Now()}
	fsys["public/home.html"] = &fstest.MapFile{Data: []byte("home updated"), ModTime: time.Now()}
	delete(fsys, "public/admin.html")

	checkInterval := fsnotify.CheckInterval
	defer func() {
		fsnotify.CheckInterval = checkInterval
	}()

	fsnotify.CheckInterval = 100 * time.Millisecond
	go app.watcher.Start()

	time.Sleep(1 * time.Second)

	req, err = http.NewRequest("GET", srv.URL+"/", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "index added", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/home.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "home updated", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin.html", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

}

func TestWatchOnHtml(t *testing.T) {
	fsys := fstest.MapFS{
		"components/header.html": {Data: []byte("<title>header</title>"), ModTime: time.Now()},
		"layouts/home.html":      {Data: []byte(`<html><head>{{ block "components/header" . }} {{end}}</head><body>{{ block "content" . }} {{end}}</body></html>`), ModTime: time.Now()},
		"views/shared.html":      {Data: []byte("<!--layout:home-->{{ define \"content\"}}<div>shared</div>{{ end }}"), ModTime: time.Now()},
		"pages/index.html":       {Data: []byte("<!--layout:home-->{{ define \"content\"}}<div>index</div>{{ end }}"), ModTime: time.Now()},
		"pages/admin/index.html": {Data: []byte("<!--layout:home-->{{ define \"content\"}}<div>admin/index</div>{{ end }}"), ModTime: time.Now()},
		"pages/admin/user.html":  {Data: []byte("<!--layout:home-->{{ define \"content\"}}<div>admin/user</div>{{ end }}"), ModTime: time.Now()},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())

	app.Get("/view", func(c *Context) error {
		return c.View(nil, "views/shared")
	})

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/about", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header</title></head><body><div>index</div></body></html>", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header</title></head><body><div>admin/index</div></body></html>", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin/user", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header</title></head><body><div>admin/user</div></body></html>", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/view", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header</title></head><body><div>shared</div></body></html>", string(buf))

	// fixed data race issue on fstest.MapFile
	app.watcher.Stop()

	fsys["components/header.html"].Data = []byte("<title>header updated</title>")
	fsys["components/header.html"].ModTime = time.Now()

	fsys["layouts/home.html"].Data = []byte(`<html><head>{{ block "components/header" . }} {{end}}</head><body>layout updated:{{ block "content" . }} {{end}}</body></html>`)
	fsys["layouts/home.html"].ModTime = time.Now()

	fsys["views/shared.html"].Data = []byte("<!--layout:home-->{{ define \"content\"}}<div>shared updated</div>{{ end }}")
	fsys["views/shared.html"].ModTime = time.Now()

	fsys["pages/index.html"].Data = []byte("<!--layout:home-->{{ define \"content\"}}<div>index updated</div>{{ end }}")
	fsys["pages/index.html"].ModTime = time.Now()

	fsys["pages/admin/index.html"].Data = []byte("<!--layout:home-->{{ define \"content\"}}<div>admin/index updated</div>{{ end }}")
	fsys["pages/admin/index.html"].ModTime = time.Now()

	// added
	fsys["pages/about.html"] = &fstest.MapFile{Data: []byte("<!--layout:home-->{{ define \"content\"}}<div>about</div>{{ end }}"), ModTime: time.Now()}

	// deleted
	delete(fsys, "pages/admin/user.html")

	checkInterval := fsnotify.CheckInterval
	fsnotify.CheckInterval = 100 * time.Millisecond
	defer func() {
		fsnotify.CheckInterval = checkInterval
	}()

	go app.watcher.Start()
	time.Sleep(1 * time.Second)

	app.watcher.Stop()

	req, err = http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header updated</title></head><body>layout updated:<div>index updated</div></body></html>", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/admin/", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header updated</title></head><body>layout updated:<div>admin/index updated</div></body></html>", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/view", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header updated</title></head><body>layout updated:<div>shared updated</div></body></html>", string(buf))

	// deleted event is not handled in html view engine, it is not updated, and does not return 404
	req, err = http.NewRequest("GET", srv.URL+"/admin/user", nil)
	req.Header.Set("Accept", "text/html")
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header</title></head><body><div>admin/user</div></body></html>", string(buf))

	// added
	req, err = http.NewRequest("GET", srv.URL+"/about", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)

	buf, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "<html><head><title>header updated</title></head><body>layout updated:<div>about</div></body></html>", string(buf))

}

type mockViewEngine struct {
}

func (*mockViewEngine) Load(fsys fs.FS, app *App) error { // skipcq: RVV-B0012
	return nil
}
func (*mockViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error { // skipcq: RVV-B0012
	return errors.New("err: unhandled error")
}

func TestHotReloadChannels(t *testing.T) {
	createApp := func(ve ...ViewEngine) *App {
		fsys := fstest.MapFS{
			"public/home.html": {Data: []byte("home"), Mode: os.ModePerm, ModTime: time.Now()},
		}

		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		defer srv.Close()
		opts := []Option{WithMux(mux), WithFsys(fsys), WithWatch()}
		if ve != nil {
			opts = append(opts, WithViewEngines(ve...))
		}
		app := New(opts...)

		app.Start()

		return app
	}

	tests := []struct {
		name       string
		createApp  func() *App
		throwError func(app *App)
	}{
		{
			name:      "should_not_panic_when_watcher_events_channel_is_closed",
			createApp: func() *App { return createApp() },
			throwError: func(app *App) {
				close(app.watcher.Events)
			},
		},
		{
			name:      "should_not_panic_when_watcher_errors_channel_is_closed",
			createApp: func() *App { return createApp() },
			throwError: func(app *App) {
				close(app.watcher.Errors)
			},
		},
		{
			name:      "should_not_panic_when_watcher_failed_to_load_views",
			createApp: func() *App { return createApp(&mockViewEngine{}) },
			throwError: func(app *App) {
				app.watcher.Events <- fsnotify.Event{Name: "public/home.html", Op: fsnotify.Write}
			},
		},
		{
			name:      "should_work_when_watcher_catch_an_error",
			createApp: func() *App { return createApp(&mockViewEngine{}) },
			throwError: func(app *App) {
				app.watcher.Errors <- errors.New("err: unhandled error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			app := tt.createApp()
			defer app.Close()

			tt.throwError(app)
		})
	}

}
