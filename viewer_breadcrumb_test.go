package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// buildBreadcrumb (unit tests on the helper directly)
// =============================================================================

func TestBuildBreadcrumbRoot(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/", nil),
	}
	require.Nil(t, buildBreadcrumb(ctx))
}

func TestBuildBreadcrumbSingleSegmentNoApp(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/blog", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Equal(t, []BreadcrumbItem{
		{Path: "/", Name: "Home"},
		{Path: "/blog", Name: "blog", Last: true},
	}, got)
}

func TestBuildBreadcrumbMultiSegment(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /blog/2026/x": {Slug: "blog/2026/x", Title: "深入 Xun"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/2026/x", nil),
	}

	got := buildBreadcrumb(ctx)
	require.Len(t, got, 4)
	require.Equal(t, BreadcrumbItem{Path: "/", Name: "Home"}, got[0])
	require.Equal(t, BreadcrumbItem{Path: "/blog", Name: "blog"}, got[1])
	require.Equal(t, BreadcrumbItem{Path: "/blog/2026", Name: "2026"}, got[2])
	require.Equal(t, BreadcrumbItem{
		Path:  "/blog/2026/x",
		Name:  "x",
		Title: "深入 Xun",
		Last:  true,
	}, got[3])
}

func TestBuildBreadcrumbTitleOnlyForMarkdownAncestors(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			// Only /blog/2026/x is markdown; intermediate paths are .html
			// pages and therefore absent from contentViews.
			"GET /blog/2026/x": {Slug: "blog/2026/x", Title: "Post Title"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/2026/x", nil),
	}

	got := buildBreadcrumb(ctx)
	for i, item := range got {
		if i == len(got)-1 {
			require.Equal(t, "Post Title", item.Title, "trailing item should pick up H1")
		} else {
			require.Empty(t, item.Title, "non-markdown ancestor %s should have empty Title", item.Path)
		}
	}
}

func TestBuildBreadcrumbMarkdownAncestorCarriesOwnH1(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /blog":           {Slug: "blog", Title: "博客首页"},
			"GET /blog/2026":      {Slug: "blog/2026", Title: "2026 年文章"},
			"GET /blog/2026/x":    {Slug: "blog/2026/x", Title: "深入 Xun"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/2026/x", nil),
	}

	got := buildBreadcrumb(ctx)
	require.Len(t, got, 4)
	require.Equal(t, "博客首页", got[1].Title)
	require.Equal(t, "2026 年文章", got[2].Title)
	require.Equal(t, "深入 Xun", got[3].Title)
}

// After the loadContentFile / loadContentPage fix, directory-level pages
// (index.md, index.html) register under the canonical /<dir>/{$} pattern.
// The URL-derived breadcrumb prefix is the no-slash form (/blog), so the
// lookup must fall back to /blog/{$}. This test pins that behavior.
func TestBuildBreadcrumbMarkdownAncestorCanonicalPattern(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /blog/{$}":     {Slug: "blog", Title: "博客首页"},
			"GET /blog/post":    {Slug: "blog/post", Title: "第一篇"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/post", nil),
	}

	got := buildBreadcrumb(ctx)
	require.Len(t, got, 3)
	require.Equal(t, "博客首页", got[1].Title,
		"the /blog ancestor must resolve via the canonical /blog/{$} pattern even though the URL prefix is /blog")
	require.Equal(t, "第一篇", got[2].Title)
}

func TestBuildBreadcrumbChineseSegmentIsRaw(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/blog/深入探索", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Len(t, got, 3)
	require.Equal(t, "深入探索", got[2].Name)
}

func TestBuildBreadcrumbDynamicSegment(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/user/42", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Len(t, got, 3)
	require.Equal(t, "42", got[2].Name, "dynamic segment should appear verbatim")
	require.True(t, got[2].Last)
	require.Empty(t, got[2].Title, "no content view registered for dynamic route")
}

func TestBuildBreadcrumbEmptyTitleIsKept(t *testing.T) {
	// A markdown file without an H1 should still produce a chain entry, just
	// without Title. The trailing item is still Last.
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /no-h1": {Slug: "no-h1", Title: ""},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/no-h1", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Len(t, got, 2)
	require.True(t, got[1].Last)
	require.Empty(t, got[1].Title)
}

// =============================================================================
// Integration: end-to-end render through HtmlViewer with real .md / .html files
// =============================================================================

func TestBreadcrumbEndToEndMarkdown(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl": {Data: []byte(`<nav>{{range .Breadcrumb}}{{if .Last}}[{{.Name}}|{{.Title}}]{{else}}<a href="{{.Path}}">{{.Name}}</a>{{end}}{{end}}</nav>`)},
		"blog/2026/x.md": {Data: []byte("# 深入 Xun\n\n正文.")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog"))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/blog/2026/x", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, `<a href="/">Home</a>`)
	require.Contains(t, body, `<a href="/blog">blog</a>`)
	require.Contains(t, body, `<a href="/blog/2026">2026</a>`)
	require.Contains(t, body, `[x|深入 Xun]`, "trailing item should render as plain with H1 title")
}

func TestBreadcrumbEndToEndHtmlPage(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/blog/index.html": {Data: []byte(`<nav>{{range .Breadcrumb}}{{if .Last}}[{{.Name}}|{{.Title}}]{{else}}<a href="{{.Path}}">{{.Name}}</a>{{end}}{{end}}</nav>`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/blog", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, `<a href="/">Home</a>`)
	require.Contains(t, body, `[blog|]`, "trailing item has no Title because no .md ancestor")
}

func TestBreadcrumbRootHasNoChain(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/index.html": {Data: []byte(`{{if .Breadcrumb}}HAS{{else}}NONE{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Equal(t, "NONE", string(buf))
}

// With the loadContentFile fix for directory-level pages, blog/index.md
// registers at "GET /blog" (not /blog/index). A child post must therefore
// pick up the directory-level H1 on the /blog ancestor of the breadcrumb chain,
// not on a hypothetical /blog/index slot that no longer exists.
func TestBreadcrumbEndToEndMarkdownIndexAncestor(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/index.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>
<nav>{{range .Breadcrumb}}{{if .Last}}[{{.Name}}|{{.Title}}]{{else}}<a href="{{.Path}}"{{with .Title}} title="{{.}}"{{end}}>{{.Name}}</a>{{end}}{{end}}</nav>`)},
		"blog/index.tpl": {Data: []byte(`<!--layout:index-->
{{define "content"}}{{.Content.Body}}{{end}}`)},
		"blog/index.md": {Data: []byte("# 博客首页\n\n这里是博客根页内容。")},
		"blog/2026/x.md": {Data: []byte("# 深入 Xun\n\n正文.")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog"))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/blog/2026/x", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, `<a href="/">Home</a>`)
	require.Contains(t, body, `<a href="/blog" title="博客首页">blog</a>`,
		"/blog ancestor must carry the directory-level page's H1 from blog/index.md")
	require.Contains(t, body, `<a href="/blog/2026">2026</a>`)
	require.Contains(t, body, `[x|深入 Xun]`, "trailing item should render as plain with H1 title")
}

// =============================================================================
// Trailing-slash normalization
// =============================================================================

func TestBuildBreadcrumbTrailingSlash(t *testing.T) {
	withSlash := buildBreadcrumb(&Context{
		Request: httptest.NewRequest(http.MethodGet, "/blog/", nil),
	})
	withoutSlash := buildBreadcrumb(&Context{
		Request: httptest.NewRequest(http.MethodGet, "/blog", nil),
	})
	require.Equal(t, withSlash, withoutSlash)
}

// =============================================================================
// Robustness: Build with nil App (defensive)
// =============================================================================

func TestBuildBreadcrumbNilApp(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/blog/2026/x", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Len(t, got, 4)
	for i, item := range got {
		if i == len(got)-1 {
			require.True(t, item.Last)
		}
		require.Empty(t, item.Title, "without App, Title stays empty")
	}
}

func TestBuildBreadcrumbNilRequest(t *testing.T) {
	require.Nil(t, buildBreadcrumb(&Context{}))
}

// =============================================================================
// ViewModel field exists with the documented zero value
// =============================================================================

func TestViewModelBreadcrumbZeroValue(t *testing.T) {
	var vm ViewModel
	require.Nil(t, vm.Breadcrumb)
	// Sanity: TempData/Data/Content also default to zero values.
	require.Nil(t, vm.TempData)
	require.Nil(t, vm.Data)
	require.Nil(t, vm.Content)
}

// =============================================================================
// Last flag is set exactly once
// =============================================================================

func TestBuildBreadcrumbExactlyOneLast(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/a/b/c/d", nil),
	}
	got := buildBreadcrumb(ctx)
	count := 0
	for _, item := range got {
		if item.Last {
			count++
		}
	}
	require.Equal(t, 1, count)
	require.True(t, got[len(got)-1].Last)
}

// =============================================================================
// Output stability: deterministic order, no surprises in serialized form
// =============================================================================

func TestBuildBreadcrumbOrderTopToBottom(t *testing.T) {
	ctx := &Context{
		Request: httptest.NewRequest(http.MethodGet, "/a/b/c", nil),
	}
	got := buildBreadcrumb(ctx)
	require.Equal(t, "/", got[0].Path)
	require.Equal(t, "/a", got[1].Path)
	require.Equal(t, "/a/b", got[2].Path)
	require.Equal(t, "/a/b/c", got[3].Path)
	// Root Name is the fixed "Home" string; subsequent Names are raw segments.
	require.Equal(t, []string{"Home", "a", "b", "c"}, []string{
		got[0].Name, got[1].Name, got[2].Name, got[3].Name,
	})
}

// =============================================================================
// Sanity: ensure no off-by-one when URL has only the root segment
// =============================================================================

func TestBuildBreadcrumbSingleSegmentIsLast(t *testing.T) {
	got := buildBreadcrumb(&Context{
		Request: httptest.NewRequest(http.MethodGet, "/only", nil),
	})
	require.Len(t, got, 2)
	require.False(t, got[0].Last)
	require.True(t, got[1].Last)
	require.Equal(t, "only", got[1].Name)
}

// =============================================================================
// Concurrent access: buildBreadcrumb is a pure function (no locks); smoke test
// confirms it composes safely under concurrent render.
// =============================================================================

func TestBuildBreadcrumbConcurrentSafe(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /x": {Title: "X"},
		},
	}
	const n = 50
	errs := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			got := buildBreadcrumb(&Context{
				App:     app,
				Request: httptest.NewRequest(http.MethodGet, "/x", nil),
			})
			if len(got) != 2 || got[1].Title != "X" || !got[1].Last {
				errs <- "wrong result"
				return
			}
			errs <- ""
		}()
	}
	for i := 0; i < n; i++ {
		if msg := <-errs; msg != "" {
			t.Fatal(msg)
		}
	}
}

// =============================================================================
// Real-world URL with %-encoded segments: framework returns decoded Path; ensure
// we handle whatever http.Request.URL.Path gives us.
// =============================================================================

func TestBuildBreadcrumbDecodedPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/blog/hello%20world", nil)
	got := buildBreadcrumb(&Context{Request: req})
	require.Len(t, got, 3)
	require.Equal(t, "hello world", got[2].Name, "URL.Path is already decoded by net/http")
	require.True(t, got[2].Last)
	require.Equal(t, "/blog/hello world", req.URL.Path, "sanity: net/http stores decoded form")
}

// =============================================================================
// Dynamic .md routes: contentViews is keyed by Routing.Pattern (with braces),
// not by the concrete URL. The current-page lookup must use Routing.Pattern
// or the H1 is lost.
// =============================================================================

func TestBuildBreadcrumbDynamicMarkdownCurrentPage(t *testing.T) {
	app := &App{
		contentViews: map[string]*ContentView{
			// Registered under the braced pattern, as the framework does.
			"GET /blog/{slug}": {Slug: "blog/{slug}", Title: "User Profile"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/foo", nil),
		Routing: Routing{Pattern: "GET /blog/{slug}"},
	}

	got := buildBreadcrumb(ctx)
	require.Len(t, got, 3)
	require.Equal(t, "foo", got[2].Name, "Name is the concrete URL segment")
	require.True(t, got[2].Last)
	require.Equal(t, "User Profile", got[2].Title, "Title must use Routing.Pattern to match braced contentViews key")
}

func TestBuildBreadcrumbDynamicMarkdownAncestorUntouched(t *testing.T) {
	// Ancestor items must still walk URL.Path; only the current page uses
	// Routing.Pattern. After the loadContentFile fix for directory-level
	// pages, blog/index.md registers at "GET /blog" (not "GET /blog/index")
	// per docs/content.md §1. So a request to /blog/index/foo walks the
	// prefix /blog first and finds the directory-level page's title; the
	// intermediate /blog/index segment has no entry because nothing is
	// registered there.
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /blog":        {Title: "Blog Index"},
			"GET /blog/{slug}": {Title: "Post Title"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/index/foo", nil),
		Routing: Routing{Pattern: "GET /blog/{slug}"},
	}

	got := buildBreadcrumb(ctx)
	require.Len(t, got, 4)
	require.Equal(t, "Blog Index", got[1].Title,
		"ancestor /blog (the directory-level page) resolves via URL-derived pattern")
	require.Empty(t, got[2].Title,
		"intermediate /blog/index has no contentView (nothing is registered at that path)")
	require.Equal(t, "Post Title", got[3].Title, "current page /blog/index/foo resolves via Routing.Pattern")
}

func TestBuildBreadcrumbDynamicMarkdownEmptyRoutingPatternFallsBack(t *testing.T) {
	// Defensive: if Routing.Pattern is empty for some reason, fall back to
	// URL-derived pattern (Title stays empty for dynamic .md, but no panic).
	app := &App{
		contentViews: map[string]*ContentView{
			"GET /blog/{slug}": {Title: "Post Title"},
		},
	}
	ctx := &Context{
		App:     app,
		Request: httptest.NewRequest(http.MethodGet, "/blog/foo", nil),
		// Routing.Pattern intentionally empty.
	}

	got := buildBreadcrumb(ctx)
	require.Empty(t, got[2].Title, "no Routing.Pattern → no Title for dynamic .md (graceful)")
	require.True(t, got[2].Last)
}
