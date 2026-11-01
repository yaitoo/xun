package xun

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yaitoo/xun/fsnotify"
)

// plainSitemapIndexTemplate is the canonical user-facing index template:
// every group becomes one <sitemap> element pointing at its child file.
const plainSitemapIndexTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
{{ range .Data -}}
  <sitemap>
    <loc>{{ .Loc }}</loc>
    <lastmod>{{ .LastMod }}</lastmod>
  </sitemap>
{{ end -}}
</sitemapindex>`

// sitemapPathGet requests the given path and returns the response and body.
func sitemapPathGet(t *testing.T, srv *httptest.Server, path string) (*http.Response, string) {
	t.Helper()

	req, err := http.NewRequest("GET", srv.URL+path, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(buf)
}

// indexTestApp builds an app with both sitemap templates published and
// serves it on a test server.
func indexTestApp(t *testing.T, opts ...Option) (*App, *httptest.Server) {
	t.Helper()

	fsys := fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}

	return newSitemapTestApp(t, fsys, opts...)
}

func TestSitemapIndex_RoutesRegistered(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "shoes.xml"}}, nil
	})

	require.Contains(t, app.Routes(), "GET /sitemap.xml")
	require.Contains(t, app.Routes(), "GET /sitemaps/{name}")

	resp, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<sitemapindex")
}

func TestSitemapIndex_IndexListsChildPaths(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "shoes.xml", URLs: []SitemapURL{{Loc: "/p/1"}}},
			{Name: "dresses.xml", URLs: []SitemapURL{{Loc: "/p/2"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/shoes.xml</loc>")
	require.Contains(t, body, "<loc>/sitemaps/dresses.xml</loc>")
	require.Equal(t, 2, strings.Count(body, "<sitemap>"))
}

func TestSitemapIndex_ChildFileServesGroupURLs(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "shoes.xml", URLs: []SitemapURL{{Loc: "/p/shoe/1"}, {Loc: "/p/shoe/2"}}},
			{Name: "dresses.xml", URLs: []SitemapURL{{Loc: "/p/dress/1"}}},
		}, nil
	})

	resp, body := sitemapPathGet(t, srv, "/sitemaps/shoes.xml")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<urlset")
	require.Contains(t, body, "<loc>/p/shoe/1</loc>")
	require.Contains(t, body, "<loc>/p/shoe/2</loc>")
	require.NotContains(t, body, "/p/dress/1")
}

func TestSitemapIndex_IndexRowCarriesNewestLastMod(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "a.xml", URLs: []SitemapURL{
				{Loc: "/1", LastMod: "2026-01-01T00:00:00Z"},
				{Loc: "/2", LastMod: "2026-02-01T00:00:00Z"},
			}},
			{Name: "b.xml", URLs: []SitemapURL{{Loc: "/3"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<lastmod>2026-02-01T00:00:00Z</lastmod>")
	require.Equal(t, 1, strings.Count(body, "<lastmod>2026-02-01T00:00:00Z</lastmod>"))
	require.Contains(t, body, "<lastmod></lastmod>",
		"a group without a LastMod contributes an empty one")
}

func TestSitemapIndex_GroupNameIsEscaped(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "女鞋", URLs: []SitemapURL{{Loc: "/p/1"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/%E5%A5%B3%E9%9E%8B</loc>")

	resp, body := sitemapPathGet(t, srv, "/sitemaps/%E5%A5%B3%E9%9E%8B")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<loc>/p/1</loc>")
}

func TestSitemapIndex_DuplicateGroupNameKeepsFirst(t *testing.T) {
	h := &captureHandler{}
	app, srv := indexTestApp(t, WithLogger(slog.New(h)))

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "a.xml", URLs: []SitemapURL{{Loc: "/first"}}},
			{Name: "a.xml", URLs: []SitemapURL{{Loc: "/second"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, 1, strings.Count(body, "<loc>/sitemaps/a.xml</loc>"))
	require.True(t, h.contains(slog.LevelWarn, "a.xml"),
		"expected a warning mentioning the duplicated group name")

	_, body = sitemapPathGet(t, srv, "/sitemaps/a.xml")
	require.Contains(t, body, "<loc>/first</loc>")
	require.NotContains(t, body, "/second")
}

func TestSitemapIndex_GroupURLsDeduped(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "a.xml", URLs: []SitemapURL{{Loc: "/dup"}, {Loc: "/dup"}, {Loc: "/other"}}},
			{Name: "b.xml", URLs: []SitemapURL{{Loc: "/dup"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemaps/a.xml")
	require.Equal(t, 1, strings.Count(body, "<loc>/dup</loc>"),
		"a duplicated Loc must be rendered exactly once per group")

	_, body = sitemapPathGet(t, srv, "/sitemaps/b.xml")
	require.Contains(t, body, "<loc>/dup</loc>",
		"the de-duplication is per group: the same Loc in two groups is published in both files")
}

func TestSitemapIndex_GroupOverCapTruncated(t *testing.T) {
	h := &captureHandler{}
	app, srv := indexTestApp(t, WithLogger(slog.New(h)))

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		urls := make([]SitemapURL, 0, sitemapEffectiveURLs+1)
		for i := 0; i <= sitemapEffectiveURLs; i++ {
			urls = append(urls, SitemapURL{Loc: fmt.Sprintf("/p/%d", i)})
		}
		return []SitemapGroup{{Name: "big.xml", URLs: urls}}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemaps/big.xml")
	require.Equal(t, sitemapEffectiveURLs, strings.Count(body, "<loc>"),
		"the child file must not exceed the effective cap")
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

func TestSitemapIndex_IndexOverCapTruncated(t *testing.T) {
	h := &captureHandler{}
	app, srv := indexTestApp(t, WithLogger(slog.New(h)))

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		groups := make([]SitemapGroup, 0, sitemapEffectiveURLs+1)
		for i := 0; i <= sitemapEffectiveURLs; i++ {
			groups = append(groups, SitemapGroup{Name: fmt.Sprintf("g%d.xml", i)})
		}
		return groups, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, sitemapEffectiveURLs, strings.Count(body, "<loc>"),
		"the index document must not exceed the effective cap")

	require.True(t, h.contains(slog.LevelWarn, "sitemap exceeds"),
		"expected a warning about the URL limit")
}

func TestSitemapIndex_NoIndexViewer_404(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	_, ok := app.viewers[sitemapIndexName]
	require.False(t, ok, "no viewer is published without public/sitemap-index.xml")

	resp, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Empty(t, body)
}

func TestSitemapIndex_NoChildViewer_404(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	resp, body := sitemapPathGet(t, srv, "/sitemaps/a.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Empty(t, body)
}

func TestSitemapIndex_BuilderError_Returns500(t *testing.T) {
	app, srv := indexTestApp(t,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return nil, errors.New("boom")
	})

	resp, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get("X-Log-Id"))
	require.NotContains(t, body, "<sitemapindex")

	resp, _ = sitemapPathGet(t, srv, "/sitemaps/a.xml")
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get("X-Log-Id"))
}

func TestSitemapIndex_UnknownGroup_404(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	resp, _ := sitemapPathGet(t, srv, "/sitemaps/missing.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSitemapIndex_GroupNameHint(t *testing.T) {
	app, srv := indexTestApp(t)

	var indexName, childName string
	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		if c.Request.PathValue("name") == "" {
			indexName = SitemapGroupName(c)
			return []SitemapGroup{
				{Name: "a.xml", URLs: []SitemapURL{{Loc: "/1"}}},
				{Name: "b.xml", URLs: []SitemapURL{{Loc: "/2"}}},
			}, nil
		}

		childName = SitemapGroupName(c)
		return []SitemapGroup{
			{Name: "a.xml", URLs: []SitemapURL{{Loc: "/1"}}},
			{Name: "b.xml", URLs: []SitemapURL{{Loc: "/2"}}},
		}, nil
	})

	_, _ = sitemapPathGet(t, srv, "/sitemap.xml")
	require.Empty(t, indexName, "the hint is empty while the index is rendered")

	_, body := sitemapPathGet(t, srv, "/sitemaps/b.xml")
	require.Equal(t, "b.xml", childName)
	require.Contains(t, body, "<loc>/2</loc>")
	require.NotContains(t, body, "<loc>/1</loc>",
		"the framework selects the group by name, even when the builder returns all of them")
}

func TestSitemapIndex_HotReloadIndexTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate), ModTime: time.Now()},
	}

	app, srv := newSitemapTestApp(t, fsys, WithWatch())

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<sitemapindex")

	// The template is published, not served as a static file.
	resp, _ := sitemapPathGet(t, srv, "/sitemap-index.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Rewriting the file re-parses it.
	fsys["public/sitemap-index.xml"] = &fstest.MapFile{
		Data:    []byte(`<sitemapindex>FIXED{{ range .Data }}<loc>{{ .Loc }}</loc>{{ end }}</sitemapindex>`),
		ModTime: time.Now(),
	}
	reload(app, fsnotify.Event{Name: "public/sitemap-index.xml", Op: fsnotify.Write})

	_, body = sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "FIXED")
	require.Contains(t, body, "/sitemaps/a.xml")

	// Deleting the file drops the viewer: the route answers 404.
	delete(fsys, "public/sitemap-index.xml")
	reload(app, fsnotify.Event{Name: "public/sitemap-index.xml", Op: fsnotify.Remove})

	_, ok := app.viewers[sitemapIndexName]
	require.False(t, ok)

	resp, _ = sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSitemapIndex_MuxAlreadyServesChildPattern(t *testing.T) {
	h := &captureHandler{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sitemaps/{name}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("raw"))
	})

	app := New(WithMux(mux), WithFsys(fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}), WithLogger(slog.New(h)))
	defer app.Close()

	// The mux already serves the pattern: registering it again would panic
	// with ServeMux's conflict panic.
	require.NotPanics(t, func() {
		app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
			return []SitemapGroup{{Name: "a.xml"}}, nil
		})
	})

	require.False(t, app.HasRoute("GET", "/sitemaps/{name}"),
		"the child route must not be registered")
	require.True(t, h.contains(slog.LevelWarn, "GET /sitemaps/{name}"),
		"expected a warning mentioning the child route")
}

func TestSitemapIndex_MuxAlreadyServesIndexPattern(t *testing.T) {
	h := &captureHandler{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("raw"))
	})

	app := New(WithMux(mux), WithFsys(fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}), WithLogger(slog.New(h)))
	defer app.Close()

	// The framework skips its default registration when the mux already
	// serves the pattern; registering it again would panic.
	require.NotPanics(t, func() {
		app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
			return []SitemapGroup{{Name: "a.xml"}}, nil
		})
	})

	require.False(t, app.HasRoute("GET", "/sitemap.xml"),
		"the index route must not be registered")
	require.True(t, h.contains(slog.LevelWarn, "GET /sitemap.xml"),
		"expected a warning mentioning the index route")
}

func TestSitemapIndex_MuxPatternOverlappingChildRoute(t *testing.T) {
	h := &captureHandler{}

	// A pattern the framework cannot see by probing: the wildcard is in the
	// first segment and a literal in the second. Registering
	// "GET /sitemaps/{name}" next to it conflicts, and the framework must
	// not crash — a page at pages/{lang}/about.html registers exactly this
	// shape.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{a}/foo", func(w http.ResponseWriter, r *http.Request) {})

	app := New(WithMux(mux), WithFsys(fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}), WithLogger(slog.New(h)))
	defer app.Close()

	require.NotPanics(t, func() {
		app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
			return []SitemapGroup{{Name: "a.xml"}}, nil
		})
	})

	require.False(t, app.HasRoute("GET", "/sitemaps/{name}"),
		"the child route must not be registered next to a conflicting pattern")
	require.True(t, h.contains(slog.LevelWarn, "GET /sitemaps/{name}"),
		"expected a warning mentioning the child route")
}

func TestSitemapIndex_MuxMethodLessPatternUnderChildRoute(t *testing.T) {
	h := &captureHandler{}

	// A method-less pattern under /sitemaps/ conflicts with the child
	// route too: the framework's pattern matches fewer methods but has a
	// more general path.
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemaps/foo", func(w http.ResponseWriter, r *http.Request) {})

	app := New(WithMux(mux), WithFsys(fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}), WithLogger(slog.New(h)))
	defer app.Close()

	require.NotPanics(t, func() {
		app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
			return []SitemapGroup{{Name: "a.xml"}}, nil
		})
	})

	require.False(t, app.HasRoute("GET", "/sitemaps/{name}"))
	require.True(t, h.contains(slog.LevelWarn, "GET /sitemaps/{name}"))
}

func TestSitemapIndex_MuxRestWildcardUnderChildRoute(t *testing.T) {
	h := &captureHandler{}

	// A rest wildcard under /sitemaps/ does NOT conflict with the child
	// route: the single-segment pattern is more specific. The framework
	// must register the child route instead of skipping it.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sitemaps/{path...}", func(w http.ResponseWriter, r *http.Request) {})

	app := New(WithMux(mux), WithFsys(fstest.MapFS{
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}), WithLogger(slog.New(h)))
	defer app.Close()

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml", URLs: []SitemapURL{{Loc: "/1"}}}}, nil
	})

	require.True(t, app.HasRoute("GET", "/sitemaps/{name}"),
		"a non-conflicting pattern must not stop the child route from being registered")
	require.False(t, h.contains(slog.LevelWarn, "GET /sitemaps/{name}"),
		"no warning is due when the pattern does not conflict")
}

func TestSitemapIndex_DotGroupNamesAreAddressable(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: ".", URLs: []SitemapURL{{Loc: "/dot"}}},
			{Name: "..", URLs: []SitemapURL{{Loc: "/dotdot"}}},
		}, nil
	})

	// "." and ".." are path segments the server resolves away, so their
	// dots must be escaped for the listed URL to reach the child route.
	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/%2E</loc>")
	require.Contains(t, body, "<loc>/sitemaps/%2E%2E</loc>")

	resp, body := sitemapPathGet(t, srv, "/sitemaps/%2E")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<loc>/dot</loc>")

	resp, body = sitemapPathGet(t, srv, "/sitemaps/%2E%2E")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<loc>/dotdot</loc>")
}

func TestSitemapIndex_GroupWithoutNameIsSkipped(t *testing.T) {
	h := &captureHandler{}
	app, srv := indexTestApp(t, WithLogger(slog.New(h)))

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "", URLs: []SitemapURL{{Loc: "/nowhere"}}},
			{Name: "a.xml", URLs: []SitemapURL{{Loc: "/1"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.NotContains(t, body, "<loc>/sitemaps/</loc>",
		"a group without a name has no addressable path")
	require.Equal(t, 1, strings.Count(body, "<sitemap>"))
	require.True(t, h.contains(slog.LevelWarn, "without a name"),
		"expected a warning about the nameless group")
}

func TestSitemapIndex_MiddlewareRunsAndAcceptIsNotNegotiated(t *testing.T) {
	app, srv := indexTestApp(t)

	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			c.WriteHeader("X-Middleware", "yes")
			return next(c)
		}
	})

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "yes", resp.Header.Get("X-Middleware"))
	require.Contains(t, string(buf), "<sitemapindex")
	require.NotContains(t, resp.Header.Get("Content-Type"), "application/json")
}

func TestSitemapIndex_SitemapRegistrationWinsLater(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml", URLs: []SitemapURL{{Loc: "/group"}}}}, nil
	})
	app.Sitemap(func(c *Context) ([]SitemapURL, error) {
		return []SitemapURL{{Loc: "/single"}}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<urlset")
	require.Contains(t, body, "/single")
	require.NotContains(t, body, "<sitemapindex")

	require.Contains(t, app.Routes(), "GET /sitemaps/{name}",
		"the child routes of the index mode stay registered")
}

func TestSitemapIndex_DefaultBuilderPublishesContentEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl":                {Data: []byte(indexTpl)},
		"content/post.md":          {Data: []byte("# P")},
		"public/sitemap.xml":       {Data: []byte(plainSitemapTemplate)},
		"public/sitemap-index.xml": {Data: []byte(plainSitemapIndexTemplate)},
	}

	app, srv := newSitemapTestApp(t, fsys)

	app.SitemapIndex()

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/content.xml</loc>")

	_, body = sitemapPathGet(t, srv, "/sitemaps/content.xml")
	require.Contains(t, body, "<loc>/content/post</loc>")
}

func TestSitemapIndex_NilBuilder_PanicsAtRegistration(t *testing.T) {
	app, _ := indexTestApp(t)

	require.PanicsWithValue(t, "xun: SitemapIndex: nil builder at index 0", func() {
		app.SitemapIndex(nil)
	})
}

func TestSitemapIndex_GroupNameWithSlashStaysOneSegment(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{
			{Name: "a/b.xml", URLs: []SitemapURL{{Loc: "/1"}}},
		}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/a%2Fb.xml</loc>")

	resp, body := sitemapPathGet(t, srv, "/sitemaps/a%2Fb.xml")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<loc>/1</loc>")
}

func TestSitemapIndex_NoFsys_RoutesRegisteredAnd404(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux))
	defer app.Close()

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "a.xml"}}, nil
	})

	require.Contains(t, app.Routes(), "GET /sitemap.xml")
	require.Contains(t, app.Routes(), "GET /sitemaps/{name}")

	resp, _ := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp, _ = sitemapPathGet(t, srv, "/sitemaps/a.xml")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSitemapIndex_EmptyGroups_RendersEmptyIndex(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) { return nil, nil })

	resp, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<sitemapindex")
	require.NotContains(t, body, "<sitemap>")
}

func TestSitemapIndex_GroupWithoutURLs_RendersEmptyUrlset(t *testing.T) {
	app, srv := indexTestApp(t)

	app.SitemapIndex(func(c *Context) ([]SitemapGroup, error) {
		return []SitemapGroup{{Name: "empty.xml"}}, nil
	})

	_, body := sitemapPathGet(t, srv, "/sitemap.xml")
	require.Contains(t, body, "<loc>/sitemaps/empty.xml</loc>")

	resp, body := sitemapPathGet(t, srv, "/sitemaps/empty.xml")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<urlset")
	require.NotContains(t, body, "<url>")
}
