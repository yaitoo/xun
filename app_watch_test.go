package xun

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun/fsnotify"
)

// reloadBarrier is an event no ViewEngine acts on: every engine ignores a
// Remove for an extension-less name, so delivering it has no side effect.
var reloadBarrier = fsnotify.Event{Name: "__reload_barrier__", Op: fsnotify.Remove}

// reload delivers events to the App's hot-reload goroutine and returns only
// once all of them have been fully processed.
//
// enableHotReload handles events strictly serially — receive, run every
// engine's FileChanged, loop back to the select — so a send that completes
// proves the *previous* event finished processing. The trailing inert
// barrier turns that into a synchronisation point: when its send returns,
// every real event ahead of it is done. That is what lets these tests assert
// immediately instead of sleeping and hoping.
func reload(app *App, events ...fsnotify.Event) {
	for _, ev := range events {
		app.watcher.Events <- ev
	}

	app.watcher.Events <- reloadBarrier
}

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

	// The poll loop is parked for the whole test binary (see TestMain), so
	// these writes cannot race a concurrent walk.
	fsys["public/index.html"] = &fstest.MapFile{Data: []byte("index added"), ModTime: time.Now()}
	fsys["public/home.html"] = &fstest.MapFile{Data: []byte("home updated"), ModTime: time.Now()}
	delete(fsys, "public/admin.html")

	reload(app,
		fsnotify.Event{Name: "public/index.html", Op: fsnotify.Create},
		fsnotify.Event{Name: "public/home.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "public/admin.html", Op: fsnotify.Remove},
	)

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

	// The poll loop is parked for the whole test binary (see TestMain), so
	// these writes cannot race a concurrent walk.
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

	// Same order the poller would emit: WalkDir visits lexically, and the
	// Remove pass over the file map runs last.
	reload(app,
		fsnotify.Event{Name: "components/header.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "layouts/home.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "pages/about.html", Op: fsnotify.Create},
		fsnotify.Event{Name: "pages/admin/index.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "pages/index.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "views/shared.html", Op: fsnotify.Write},
		fsnotify.Event{Name: "pages/admin/user.html", Op: fsnotify.Remove},
	)

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

func (*mockViewEngine) Load(fsys fs.FS, app *App) { // skipcq: RVV-B0012

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
			// Stop is the only way Events/Errors close now: Start owns
			// both channels and closes them on its way out. Closing them
			// from here, as this test used to, would race the sender.
			name:      "should_not_panic_when_watcher_is_stopped",
			createApp: func() *App { return createApp() },
			throwError: func(app *App) {
				app.watcher.Stop()
			},
		},
		{
			name:      "should_not_panic_when_watcher_is_stopped_twice",
			createApp: func() *App { return createApp() },
			throwError: func(app *App) {
				app.watcher.Stop()
				app.watcher.Stop()
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

// TestCloseStopsWatcherGoroutines is the regression test for #132: an App
// that opted into WithWatch used to keep both hot-reload goroutines — and the
// poll loop's fs walk — alive for the rest of the process, because Close did
// nothing and nothing ever closed the watcher's channels.
//
// The assertion is the synctest bubble itself: Test waits for every goroutine
// started inside it to exit, and fails on deadlock. So a watcher that outlives
// Close fails the test directly, with no goroutine counting, no slack for
// scheduler noise, and no interference from other tests. Against the old
// implementation this test deadlocked; the goroutine-counting version it
// replaces reported "2 before, 42 after" for the loop below.
//
// No HTTP here on purpose: real socket I/O is not durably blocking, so a
// bubble and httptest do not mix. The reload behaviour is covered by
// TestWatchOnStatic and friends, which stay outside a bubble.
func TestCloseStopsWatcherGoroutines(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for range 5 {
			fsys := fstest.MapFS{
				"public/home.html": {Data: []byte("home"), Mode: os.ModePerm, ModTime: time.Now()},
			}

			// A fresh mux per iteration: New falls back to
			// http.DefaultServeMux, and re-registering the same pattern on it
			// panics.
			app := New(WithMux(http.NewServeMux()), WithFsys(fsys), WithWatch())
			app.Start()

			app.Close()
			app.Close() // idempotent

			// Converge before the next iteration so a leak is attributed to
			// the App that caused it rather than to the last one.
			synctest.Wait()
		}
	})
}

// TestCloseWithoutWatchIsNoop pins that Close stays safe on an App that never
// opted into WithWatch, where app.watcher is nil.
func TestCloseWithoutWatchIsNoop(t *testing.T) {
	app := New(WithMux(http.NewServeMux()))

	require.NotPanics(t, func() {
		app.Close()
		app.Close()
	})
}

// unwalkableFS fails every Open, which is enough to make fs.WalkDir — and so
// Watcher.Add — return an error. It stands in for a real WithFsys pointed at a
// directory that does not exist.
type unwalkableFS struct{}

func (unwalkableFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// TestCloseAfterWatcherAddFailed covers app.go's watcher-add error branch,
// where the App keeps a non-nil watcher but never starts one: New logs the
// failure and skips enableHotReload, so nothing is ever received on the
// watcher's done channel.
//
// That combination is what makes Close interesting here. Stop signals by
// closing done rather than sending on it, so it does not need a receiver — but
// the obvious "fix" for #132, a blocking send, would hang the caller forever on
// exactly this path. The synctest bubble is the assertion: a Close that blocks
// leaves the root goroutine stuck with nothing to wake it, which fails as a
// deadlock.
//
// The empty WithViewEngines keeps the engines away from the broken fs so this
// stays a test about the watcher branch alone.
func TestCloseAfterWatcherAddFailed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		app := New(
			WithMux(http.NewServeMux()),
			WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
			WithFsys(unwalkableFS{}),
			WithWatch(),
			WithViewEngines(),
		)

		require.NotNil(t, app.watcher, "New must still hold the watcher after Add fails")

		app.Close()
		app.Close() // idempotent
	})
}
