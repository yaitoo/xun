package xun

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// captureHandler records slog messages so a test can assert on them.
type captureHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, r.Message)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// contains reports whether any recorded message contains substr.
func (h *captureHandler) contains(substr string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, m := range h.messages {
		if strings.Contains(m, substr) {
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
			return []SitemapURL{{Loc: "/a"}, {Loc: "/b"}}, nil
		},
		func(c *Context) ([]SitemapURL, error) {
			return []SitemapURL{{Loc: "/b"}, {Loc: "/c"}}, nil
		},
	)

	_, body := sitemapGet(t, srv, nil)
	require.Contains(t, body, "<loc>/a</loc>")
	require.Contains(t, body, "<loc>/b</loc>")
	require.Contains(t, body, "<loc>/c</loc>")
	require.Equal(t, 1, strings.Count(body, "<loc>/b</loc>"),
		"a duplicated Loc must be rendered exactly once")

	// The order is the builders' order, not sorted.
	require.Less(t, strings.Index(body, "/a"), strings.Index(body, "/b"))
	require.Less(t, strings.Index(body, "/b"), strings.Index(body, "/c"))
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

	require.False(t, h.contains("GET /sitemap.xml"),
		"registering a route must not warn on its own")

	app.Sitemap(func(c *Context) ([]SitemapURL, error) { return nil, nil })

	require.True(t, h.contains("GET /sitemap.xml"),
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

	require.False(t, h.contains("GET /sitemap.xml"),
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
