package xun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yaitoo/xun/fsnotify"
)

// newSitemapTestApp builds an app backed by fsys and serves it on a test
// server. The caller registers routes / builders on the returned app.
func newSitemapTestApp(t *testing.T, fsys fs.FS, opts ...Option) (*App, *httptest.Server) {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	opts = append([]Option{WithMux(mux), WithFsys(fsys)}, opts...)
	app := New(opts...)
	t.Cleanup(app.Close)
	app.Start()

	return app, srv
}

// sitemapGet requests GET /sitemap.xml and returns the response and body.
func sitemapGet(t *testing.T, srv *httptest.Server, header http.Header) (*http.Response, string) {
	t.Helper()

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(buf)
}

// captureHandler records slog records so a test can assert on them.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// contains reports whether a record logged at the given level has a message
// containing substr. The level matters: a message logged at Info is not a
// warning.
func (h *captureHandler) contains(level slog.Level, substr string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, r := range h.records {
		if r.Level == level && strings.Contains(r.Message, substr) {
			return true
		}
	}
	return false
}

func TestSitemapApp_DefaultRouteRegisteredAndRunsMiddleware(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			c.WriteHeader("X-Middleware", "yes")
			return next(c)
		}
	})
	app.Start()
	defer app.Close()

	require.True(t, app.HasRoute("GET", "/sitemap.xml"))
	require.Contains(t, app.Routes(), "GET /sitemap.xml")

	resp, body := sitemapGet(t, srv, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "yes", resp.Header.Get("X-Middleware"))
	require.Contains(t, body, "<urlset")
}

func TestSitemapApp_NoFsys_NoRouteRegistered(t *testing.T) {
	mux := http.NewServeMux()
	app := New(WithMux(mux))
	defer app.Close()

	require.False(t, app.HasRoute("GET", "/sitemap.xml"))
	require.NotContains(t, app.Routes(), "GET /sitemap.xml")
}

func TestSitemapApp_DefaultBuilderRendersContentEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	// No builder: the content engine's entries are used.
	app.Sitemap()

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/content/post</loc>")
}

func TestSitemapApp_BuilderEntriesRendered(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		return []SitemapURL{{Loc: "/db/1", LastMod: "2026-01-02T03:04:05Z"}}, nil
	})

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/db/1</loc>")
	require.Contains(t, body, "<lastmod>2026-01-02T03:04:05Z</lastmod>")
}

func TestSitemapApp_MultipleBuildersConcatenatedAndDeduped(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Sitemap(
		func(c *Context) ([]SitemapURL, error) {
			return []SitemapURL{
				{Loc: "/c"},
				{Loc: "/b", LastMod: "2026-01-01T00:00:00Z"},
			}, nil
		},
		func(c *Context) ([]SitemapURL, error) {
			return []SitemapURL{
				{Loc: "/a"},
				{Loc: "/b", LastMod: "2026-06-06T00:00:00Z"},
			}, nil
		},
	)

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/a</loc>")
	require.Contains(t, body, "<loc>/b</loc>")
	require.Contains(t, body, "<loc>/c</loc>")
	require.Equal(t, 1, strings.Count(body, "<loc>/b</loc>"),
		"a duplicated Loc must be rendered exactly once")

	// The first occurrence wins, so the first builder's values survive.
	require.Contains(t, body, "<lastmod>2026-01-01T00:00:00Z</lastmod>")
	require.NotContains(t, body, "2026-06-06T00:00:00Z")

	// The order is the builders' order (c, b, a), not sorted (a, b, c).
	require.Less(t, strings.Index(body, "/c"), strings.Index(body, "/b"))
	require.Less(t, strings.Index(body, "/b"), strings.Index(body, "/a"))
}

func TestSitemapApp_TempDataFromBuilderAvailableToTemplate(t *testing.T) {
	const tpl = `<?xml version="1.0" encoding="UTF-8"?>
<urlset>
  <site>{{ .TempData.site }}</site>
{{ range .Data -}}
  <url><loc>{{ .Loc }}</loc></url>
{{ end -}}
</urlset>`

	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(tpl)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		c.Set("site", "example.com")
		return []SitemapURL{{Loc: "/a"}}, nil
	})

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "example.com")
	require.Contains(t, body, "<loc>/a</loc>")
}

func TestSitemapApp_ViewerResolvedPerRequest(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	_, first := sitemapGet(t, srv, nil)
	require.Contains(t, first, "<urlset")

	// Replace the published viewer. The route resolves the viewer on every
	// request, so the next response must use the new one — without the
	// route being re-registered.
	other := &TextTemplate{name: "other.xml"}
	require.NoError(t, other.Load(
		fstest.MapFS{"other.xml": {Data: []byte(`<urlset>OTHER</urlset>`)}},
		template.FuncMap{},
	))
	app.viewers[sitemapName] = NewTextViewer(other)

	_, second := sitemapGet(t, srv, nil)
	require.Contains(t, second, "OTHER")
}

func TestSitemapApp_NoTemplate_404(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# P")},
	}

	app, srv := newSitemapTestApp(t, fsys)

	_, ok := app.viewers[sitemapName]
	require.False(t, ok, "no viewer is published without public/sitemap.xml")

	resp, body := sitemapGet(t, srv, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Empty(t, body)
}

func TestSitemapApp_EmptyEntries_RendersEmptyUrlset(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Sitemap(func(c *Context) ([]SitemapURL, error) { return nil, nil })

	resp, body := sitemapGet(t, srv, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<urlset")
	require.NotContains(t, body, "<url>")
}

func TestSitemapApp_AcceptJSON_StillRendersTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	_, srv := newSitemapTestApp(t, fsys)

	header := http.Header{"Accept": []string{"application/json"}}
	resp, body := sitemapGet(t, srv, header)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<urlset")
	require.NotContains(t, resp.Header.Get("Content-Type"), "application/json")
}

func TestSitemapApp_LastRegistrationWins(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Get("/sitemap.xml", func(c *Context) error {
		c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err := c.Response.Write([]byte("user"))
		return err
	})

	_, body := sitemapGet(t, srv, nil)
	require.Equal(t, "user", body)

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		return []SitemapURL{{Loc: "/x"}}, nil
	})

	_, body = sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/x</loc>")
}

func TestSitemapApp_WarnsWhenReplacingUserRoute(t *testing.T) {
	h := &captureHandler{}
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, _ := newSitemapTestApp(t, fsys, WithLogger(slog.New(h)))

	app.Get("/sitemap.xml", func(c *Context) error {
		return c.View(c.App.SitemapURLs(), sitemapName)
	})

	require.False(t, h.contains(slog.LevelWarn, "GET /sitemap.xml"),
		"registering a route must not warn on its own")

	app.Sitemap(func(c *Context) ([]SitemapURL, error) { return nil, nil })

	require.True(t, h.contains(slog.LevelWarn, "GET /sitemap.xml"),
		"expected a warning mentioning GET /sitemap.xml")
}

func TestSitemapApp_RepeatedSitemapCallDoesNotWarn(t *testing.T) {
	h := &captureHandler{}
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, _ := newSitemapTestApp(t, fsys, WithLogger(slog.New(h)))

	app.Sitemap()
	app.Sitemap()

	require.False(t, h.contains(slog.LevelWarn, "GET /sitemap.xml"),
		"the sitemap route is owned by the framework; replacing it is not a warning")
}

func TestSitemapApp_EntryRenderedVerbatim(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		return []SitemapURL{
			{Loc: "https://other.example/p/1"},
			{Loc: "/relative"},
		}, nil
	})

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>https://other.example/p/1</loc>",
		"the framework must not resolve or rewrite a Loc")
	require.Contains(t, body, "<loc>/relative</loc>")
}

func TestSitemapApp_UserHandlerRendersNamedViewer(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.Get("/sitemap.xml", func(c *Context) error {
		return c.View(c.App.SitemapURLs(), sitemapName)
	})

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/content/post</loc>")
}

func TestSitemapApp_BuilderError_Returns500(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		return nil, errors.New("boom")
	})

	resp, body := sitemapGet(t, srv, nil)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get("X-Log-Id"))
	require.NotContains(t, body, "<urlset")
}

func TestSitemapApp_GroupRouteIsUserOwned(t *testing.T) {
	h := &captureHandler{}
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, _ := newSitemapTestApp(t, fsys, WithLogger(slog.New(h)))

	// A group registers through app.createHandler, not app.HandleFunc —
	// the ownership hand-off must cover it too.
	app.Group("").Get("/sitemap.xml", func(c *Context) error {
		return c.View(c.App.SitemapURLs(), sitemapName)
	})

	app.Sitemap(func(c *Context) ([]SitemapURL, error) { return nil, nil })

	require.True(t, h.contains(slog.LevelWarn, "GET /sitemap.xml"),
		"expected a warning: the group-owned route is being replaced")
}

func TestSitemapApp_NilBuilder_PanicsAtRegistration(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, _ := newSitemapTestApp(t, fsys)

	// A nil builder must fail at registration, not inside the request
	// goroutine where the stack trace points at the framework.
	require.PanicsWithValue(t, "xun: Sitemap: nil builder at index 0", func() {
		app.Sitemap(nil)
	})
}

func TestSitemapApp_MuxPatternOwnedByFramework(t *testing.T) {
	mux := http.NewServeMux()
	app := New(WithMux(mux), WithFsys(fstest.MapFS{}))
	defer app.Close()

	// The framework registers "GET /sitemap.xml" on the mux whenever a
	// file system is configured, so the raw escape hatch cannot claim that
	// pattern — ServeMux panics on a duplicate registration. Users take
	// the route over with app.Get / app.Sitemap instead.
	require.Panics(t, func() {
		app.Mux().HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {})
	})
}

func TestSitemapApp_MuxAlreadyOwnsPattern(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("raw"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// The mux already serves the pattern: New must neither register it
	// again (ServeMux panics on a duplicate pattern) nor clobber the
	// existing handler.
	app := New(WithMux(mux), WithFsys(fstest.MapFS{}))
	defer app.Close()

	require.False(t, app.HasRoute("GET", "/sitemap.xml"))

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "raw", string(buf))
}

func TestSitemapApp_HotAddedPageTakesOver(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys, WithWatch())

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<urlset")

	// A page registered for the same route takes it over — the same
	// precedence the page gets at startup, where New() leaves an existing
	// route alone.
	fsys["pages/sitemap.xml.html"] = &fstest.MapFile{
		Data:    []byte("<h1>Page</h1>"),
		ModTime: time.Now(),
	}
	reload(app, fsnotify.Event{Name: "pages/sitemap.xml.html", Op: fsnotify.Create})

	_, body = sitemapGet(t, srv, nil)
	require.Contains(t, body, "<h1>Page</h1>")
}

func TestSitemapApp_TooManyURLs_WarnsAndTruncates(t *testing.T) {
	h := &captureHandler{}
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys, WithLogger(slog.New(h)))

	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		urls := make([]SitemapURL, 0, sitemapEffectiveURLs+1)
		for i := 0; i <= sitemapEffectiveURLs; i++ {
			urls = append(urls, SitemapURL{Loc: fmt.Sprintf("/p/%d", i)})
		}
		return urls, nil
	})

	_, body := sitemapGet(t, srv, nil)

	// A crawler rejects a sitemap that exceeds the limit as a whole, so
	// every URL in it would be lost. The framework truncates at the
	// effective cap and logs what it dropped.
	require.Equal(t, sitemapEffectiveURLs, strings.Count(body, "<loc>"),
		"the served document must not exceed the effective cap")
	require.NotContains(t, body, fmt.Sprintf("/p/%d", sitemapEffectiveURLs))

	require.True(t, h.contains(slog.LevelWarn, "sitemap exceeds"),
		"expected a warning about the URL limit")
	require.True(t, h.contains(slog.LevelWarn, strconv.Itoa(sitemapEffectiveURLs+1)),
		"expected the warning to carry the total")
	require.True(t, h.contains(slog.LevelWarn, strconv.Itoa(sitemapEffectiveURLs)),
		"expected the warning to carry the cap")
	require.True(t, h.contains(slog.LevelWarn, "1 dropped"),
		"expected the warning to carry the dropped count")
}

func TestSitemapApp_StaticEnginePublishesViewerWithoutRoute(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app := New(WithMux(http.NewServeMux()))
	defer app.Close()

	ve := &StaticViewEngine{}
	ve.Load(fsys, app)

	require.Empty(t, app.routes, "the static view engine must not register routes")
	_, ok := app.viewers[sitemapName]
	require.True(t, ok, "the sitemap template must be published as a viewer")
}

func TestSitemapApp_FileChangeKeepsRouteRegistration(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate), ModTime: time.Now()},
	}

	app, srv := newSitemapTestApp(t, fsys, WithWatch())

	route := app.routes[sitemapKey]
	require.NotNil(t, route)

	// Fix the template.
	fsys["public/sitemap.xml"] = &fstest.MapFile{
		Data:    []byte(`<urlset>FIXED{{ range .Data }}<url><loc>{{ .Loc }}</loc></url>{{ end }}</urlset>`),
		ModTime: time.Now(),
	}
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Write})

	require.Same(t, route, app.routes[sitemapKey],
		"a file change must refresh the viewer, not re-register the route")

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "FIXED")
	require.Contains(t, body, "<loc>/content/post</loc>")
}
