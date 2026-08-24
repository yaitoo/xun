package xun

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yaitoo/xun/fsnotify"
)

// =============================================================================
// contentRenderer.Extract
// =============================================================================

func TestExtractFirstH1(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`# Hello World

Some paragraph.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Hello World", title)
	require.Equal(t, "Some paragraph.", desc) // paragraph after title becomes description
}

func TestExtractIgnoresH1InCodeBlock(t *testing.T) {
	r := newContentRenderer()

	content := []byte("```\n# Not A Title\n```\n\n# Real Title\n")

	title, _ := r.Extract(content)
	require.Equal(t, "Real Title", title)
}

func TestExtractIgnoresH2AndBelow(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`## Sub Heading

# Real Title

### Deeper
`)

	title, _ := r.Extract(content)
	require.Equal(t, "Real Title", title)
}

func TestExtractNoH1(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`## Only subheadings here.

Plain text.
`)

	title, _ := r.Extract(content)
	require.Empty(t, title)
}

func TestExtractBlockquotePreferredOverParagraph(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`# Title

> This is a blockquote summary.

This is a paragraph.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Title", title)
	require.Equal(t, "This is a blockquote summary.", desc)
}

func TestExtractFallsBackToParagraph(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`# Title

This is the first paragraph.
Second line of paragraph.

Another paragraph below.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Title", title)
	require.Equal(t, "This is the first paragraph.\nSecond line of paragraph.", desc)
}

func TestExtractStopsAtSubheading(t *testing.T) {
	// Once an H2 or deeper heading appears, the description window closes.
	r := newContentRenderer()

	content := []byte(`# Title

## Subheading

More content.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Title", title)
	require.Empty(t, desc)
}

func TestExtractIgnoresNestedParagraphs(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`# Title

- list item with nested para

  > nested blockquote

> Top-level blockquote wins.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Title", title)
	// First paragraph encountered is the one inside the list's nested blockquote,
	// which is a child of a Blockquote; this counts as a blockquote lede and wins.
	require.Equal(t, "nested blockquote", desc)
}

func TestExtractNoDescription(t *testing.T) {
	r := newContentRenderer()

	content := []byte(`# Title

## Subheading

More content.
`)

	title, desc := r.Extract(content)
	require.Equal(t, "Title", title)
	require.Empty(t, desc)
}

func TestExtractEmptyContent(t *testing.T) {
	r := newContentRenderer()

	title, desc := r.Extract(nil)
	require.Empty(t, title)
	require.Empty(t, desc)
}

// =============================================================================
// contentRenderer.Render
// =============================================================================

func TestRenderBasicMarkdown(t *testing.T) {
	r := newContentRenderer()

	html, err := r.Render([]byte("# Hello\n\nWorld"))
	require.NoError(t, err)
	require.Contains(t, string(html), "<h1>Hello</h1>")
	require.Contains(t, string(html), "<p>World</p>")
}

func TestRenderEmptyContent(t *testing.T) {
	r := newContentRenderer()

	html, err := r.Render(nil)
	require.NoError(t, err)
	require.Empty(t, string(html))
}

func TestRenderStripsRawHtmlByDefault(t *testing.T) {
	// Default goldmark is safe-mode: raw HTML is stripped, not rendered.
	// To allow raw HTML, users must provide a custom renderer via
	// WithContentRenderer configured with html.WithUnsafe().
	r := newContentRenderer()

	html, err := r.Render([]byte("<script>alert(1)</script>"))
	require.NoError(t, err)
	require.NotContains(t, string(html), "<script>alert(1)</script>")
}

func TestRenderGFMTable(t *testing.T) {
	r := newContentRenderer()

	html, err := r.Render([]byte("| a | b |\n|---|---|\n| 1 | 2 |"))
	require.NoError(t, err)
	require.Contains(t, string(html), "<table>")
	require.Contains(t, string(html), "<td>1</td>")
}

// =============================================================================
// extractContentView
// =============================================================================

func TestExtractContentViewFromPath(t *testing.T) {
	r := newContentRenderer()
	date := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	fi := &fakeFileInfo{name: "hello.md", modTime: date}

	cv := extractContentView("content/hello.md", []byte("# Hello"), fi, "content", r)

	require.Equal(t, "content/hello.md", cv.Path)
	require.Equal(t, "hello", cv.Slug)
	require.Equal(t, "Hello", cv.Title)
	require.Equal(t, date, cv.Date)
	require.Empty(t, cv.Body)
}

func TestExtractContentViewNestedSlug(t *testing.T) {
	r := newContentRenderer()
	fi := &fakeFileInfo{name: "deeper.md"}

	cv := extractContentView("content/2026/deeper.md", []byte("# Deeper"), fi, "content", r)

	require.Equal(t, "2026/deeper", cv.Slug)
}

func TestExtractContentViewFallbackTitle(t *testing.T) {
	r := newContentRenderer()
	fi := &fakeFileInfo{name: "no-heading.md"}

	cv := extractContentView("content/no-heading.md", []byte("No heading here"), fi, "content", r)

	// No H1 → Title left empty per spec; callers guard with {{if .Title}}.
	require.Empty(t, cv.Title)
}

func TestExtractContentViewNilFileInfo(t *testing.T) {
	r := newContentRenderer()

	cv := extractContentView("content/x.md", []byte("# X"), nil, "content", r)

	require.True(t, cv.Date.IsZero())
}

// =============================================================================
// End-to-end: loadContentDir + bubbleUp + Render
// =============================================================================

func TestContentEngineEndToEnd(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html><body>{{block "content" .}}{{end}}</body></html>`)},
		"content/hello.md":  {Data: []byte("# Hello\n\nWorld from markdown.")},
		"content/2026/deeper.md": {
			Data: []byte(`# Deeper Post

> A lede paragraph.

## Section

More content.`),
		},
		"content/2026/index.tpl": {Data: []byte(`<!--layout:site-->
{{define "content"}}<article><h1>{{.Content.Title}}</h1><p>{{.Content.Description}}</p>{{.Content.Body}}</article>{{end}}`)},
		"content/index.tpl":       {Data: []byte(`<!--layout:site-->
{{define "content"}}<h1>{{.Content.Title}}</h1>{{.Content.Body}}{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/hello", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, "Hello")              // title from H1
	require.Contains(t, body, "World from markdown") // rendered body
	require.Contains(t, body, "<html>")             // layout wrapper

	req, _ = http.NewRequest("GET", srv.URL+"/content/2026/deeper", nil)
	req.Header.Set("Accept", "text/html")
	resp, err = client.Do(req)
	require.NoError(t, err)
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	body = string(buf)
	require.Contains(t, body, "Deeper Post")             // title
	require.Contains(t, body, "A lede paragraph.")       // description (blockquote)
	require.Contains(t, body, "<article>")               // bubble-up to 2026/index.tpl
	require.Contains(t, body, "<h2>Section</h2>")         // markdown h2 in body
	require.Contains(t, body, "More content.")            // markdown paragraph
}

func TestContentEngineBubbleUpToRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/orphan.md": {Data: []byte("# Orphan")},
		"index.tpl": {Data: []byte(`<!--layout:site-->
{{define "content"}}<main>{{.Content.Title}}{{.Content.Body}}</main>{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/orphan", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "<main>")
	require.Contains(t, string(buf), "Orphan")
}

func TestContentEngineNoTemplate(t *testing.T) {
	// No index.tpl anywhere → warning logged, route not registered.
	fsys := fstest.MapFS{
		"content/orphan.md": {Data: []byte("# Orphan")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/orphan", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestContentEngineNonContentRouteHasNilContent(t *testing.T) {
	// Plain page (no .md) should have vm.Content == nil.
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"pages/about.html": {Data: []byte(`<!--layout:site-->
{{define "content"}}<h1>About</h1>{{if .Content}}<p>BAD</p>{{end}}{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/about", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, "<h1>About</h1>")
	require.NotContains(t, body, "BAD") // Content was nil → guard skipped
}

// A bare .tpl template in content/ must NOT register a route. Only the
// template is loaded into the template graph.
func TestContentTemplateNotRegistered(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {Data: []byte(`<!--layout:site-->
{{define "content"}}<p>template only</p>{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/blog", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		".tpl must not register a route; only sibling .md does")
}

// .tpl (template) and .md (page) coexist in the same directory. The
// template wraps sibling .md posts; the .md file is itself a real page
// at the directory route.
func TestContentTemplateAndPageCoexist(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {Data: []byte(`<!--layout:site-->
{{define "content"}}<article><h1>{{.Content.Title}}</h1><div>{{.Content.Body}}</div></article>{{end}}`)},
		"content/blog/index.md": {Data: []byte("# 博客首页\n\n这里是博客根页内容。")},
		"content/blog/post.md": {Data: []byte("# 第一篇\n\n文章正文。")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	// Directory-level page: GET /content/blog/index renders index.md wrapped by index.tpl.
	req, _ := http.NewRequest("GET", srv.URL+"/content/blog/index", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(buf)
	require.Contains(t, body, "<article>")
	require.Contains(t, body, "博客首页")
	require.Contains(t, body, "这里是博客根页内容")

	// Sibling post: GET /content/blog/post uses the same template.
	req, _ = http.NewRequest("GET", srv.URL+"/content/blog/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err = client.Do(req)
	require.NoError(t, err)
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(buf)
	require.Contains(t, body, "<article>")
	require.Contains(t, body, "第一篇")

	// Both routes should have their own ContentView so the breadcrumb
	// chain can carry H1s for the ancestor directory.
	require.Contains(t, app.contentViews, "GET /content/blog/index")
	require.Contains(t, app.contentViews, "GET /content/blog/post")
	require.Equal(t, "博客首页", app.contentViews["GET /content/blog/index"].Title)
}

// .html in content/ is a page route only — it must NOT be picked up by
// bubbleUp even if it sits next to sibling .md files.
func TestContentHtmlNotBubbleUp(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/foo.md": {Data: []byte("# Foo")},
		"content/blog/index.html": {Data: []byte(`<!--layout:site-->
{{define "content"}}<p>standalone html page, not a template</p>{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	// /blog is registered as a page route via the .html file (the
	// content/ prefix is stripped because loadContentPage registers
	// the page from the in-content relative path).
	req, _ := http.NewRequest("GET", srv.URL+"/blog", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Contains(t, string(buf), "standalone html page")

	// /content/blog/foo has no bubble-up template → route not registered → 404.
	req, _ = http.NewRequest("GET", srv.URL+"/content/blog/foo", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		".html in content/ is a page, not a bubble-up template")
}

// index.tpl (bubble-up template) and index.html (standalone page) must
// coexist in the same directory. They are two distinct templates with
// distinct roles — the template key must NOT collide.
func TestContentTplAndHtmlCoexistInSameDir(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {Data: []byte(`<!--layout:site-->
{{define "content"}}<article>WRAP {{.Content.Body}}</article>{{end}}`)},
		"content/blog/index.html": {Data: []byte(`<!--layout:site-->
{{define "content"}}<p>standalone html page</p>{{end}}`)},
		"content/blog/post.md": {Data: []byte("# Post\n\nBody text")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	// Both templates must be cached under distinct keys. The bug being
	// guarded against: stripping the extension produced identical keys
	// ("content/blog/index") for both files, so whichever was walked
	// last would silently overwrite the other.
	var ve *HtmlViewEngine
	for _, e := range app.engines {
		if hve, ok := e.(*HtmlViewEngine); ok {
			ve = hve
			break
		}
	}
	require.NotNil(t, ve)
	require.Contains(t, ve.templates, "content/blog/index.tpl",
		"index.tpl must be loaded as a separate template")
	require.Contains(t, ve.templates, "content/blog/index.html",
		"index.html must be loaded as a separate template")
	tplT := ve.templates["content/blog/index.tpl"]
	tplH := ve.templates["content/blog/index.html"]
	require.NotSame(t, tplT, tplH,
		"the two templates must NOT share the same *HtmlTemplate instance")

	// /blog is the standalone .html page.
	req, _ := http.NewRequest("GET", srv.URL+"/blog", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Contains(t, string(buf), "standalone html page")

	// /content/blog/post is the .md sibling, wrapped by the .tpl template
	// (NOT the .html page). If the keys collided, the post would render
	// the standalone .html instead of the wrapping .tpl.
	req, _ = http.NewRequest("GET", srv.URL+"/content/blog/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err = client.Do(req)
	require.NoError(t, err)
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(buf)
	require.Contains(t, body, "WRAP", "post must be wrapped by index.tpl")
	require.Contains(t, body, "Body text")
	require.NotContains(t, body, "standalone html page",
		"post must NOT be rendered through the index.html page template")
}

// =============================================================================
// bubbleUp unit
// =============================================================================

func TestBubbleUpSpecific(t *testing.T) {
	fsys := fstest.MapFS{
		"content/2026/deeper.tpl": {Data: []byte(`<p>specific</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	require.Equal(t, "content/2026/deeper.tpl", ve.bubbleUp("content/2026/deeper.md"))
}

func TestBubbleUpToIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"content/2026/index.tpl": {Data: []byte(`<p>index</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	require.Equal(t, "content/2026/index.tpl", ve.bubbleUp("content/2026/deeper.md"))
}

func TestBubbleUpToContentIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"content/index.tpl": {Data: []byte(`<p>index</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	require.Equal(t, "content/index.tpl", ve.bubbleUp("content/anything.md"))
}

func TestBubbleUpToRootIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl": {Data: []byte(`<p>root</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	require.Equal(t, "index.tpl", ve.bubbleUp("content/anything.md"))
}

func TestBubbleUpNotFound(t *testing.T) {
	fsys := fstest.MapFS{}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	require.Equal(t, "", ve.bubbleUp("anything"))
}

// =============================================================================
// Options
// =============================================================================

func TestWithContentSingle(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/post.md": {Data: []byte("# Hello")},
		"index.tpl":    {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("docs"))
	app.Close()

	// Directory name becomes the URL prefix.
	req, _ := http.NewRequest("GET", srv.URL+"/docs/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "Hello")
}

func TestWithContentMultiple(t *testing.T) {
	fsys := fstest.MapFS{
		"blog/post.md":      {Data: []byte("# Blog Post")},
		"docs/api/intro.md": {Data: []byte("# Intro")},
		"kb/123.md":         {Data: []byte("# KB Article")},
		"index.tpl":         {Data: []byte(`<html>{{.Content.Title}}</html>`)},
		"blog/index.tpl":    {Data: []byte(`<html>blog: {{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog", "docs", "kb"))
	app.Close()

	cases := []struct{ url, want string }{
		{"/blog/post", "Blog Post"},
		{"/docs/api/intro", "Intro"},
		{"/kb/123", "KB Article"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest("GET", srv.URL+tc.url, nil)
		req.Header.Set("Accept", "text/html")
		resp, err := client.Do(req)
		require.NoError(t, err)
		buf, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Contains(t, string(buf), tc.want, "url=%s", tc.url)
	}

	// No collision: blog/post and docs/api/intro both have unique patterns.
	require.NotEmpty(t, app.contentViews["GET /blog/post"])
	require.NotEmpty(t, app.contentViews["GET /docs/api/intro"])
	require.Empty(t, app.contentViews["GET /docs/post"]) // doesn't exist
}

func TestWithContentAccumulates(t *testing.T) {
	fsys := fstest.MapFS{
		"blog/a.md": {Data: []byte("# A")},
		"docs/b.md": {Data: []byte("# B")},
		"index.tpl": {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog"), WithContent("docs"))
	app.Close()

	require.NotEmpty(t, app.contentViews["GET /blog/a"])
	require.NotEmpty(t, app.contentViews["GET /docs/b"])
}

func TestWithContentEmptyDisables(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Hello")},
		"index.tpl":       {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent(""))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	// Route not registered → 404
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWithContentRenderer(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Hello")},
		"index.tpl":       {Data: []byte(`<html>{{.Content.Body}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(
		WithMux(mux),
		WithFsys(fsys),
		WithContentRenderer(func(content []byte, path string) (template.HTML, error) {
			return template.HTML("<p>CUSTOM-RENDERED</p>"), nil
		}),
	)
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "CUSTOM-RENDERED")
}

func TestWithContentMeta(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Whatever")},
		"index.tpl":       {Data: []byte(`<html><h1>{{.Content.Title}}</h1></html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(
		WithMux(mux),
		WithFsys(fsys),
		WithContentMeta(func(path string, content []byte, fi fs.FileInfo) ContentView {
			return ContentView{
				Path:        path,
				Slug:        "post",
				Title:       "CUSTOM-TITLE",
				Description: "from meta hook",
			}
		}),
	)
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/content/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "CUSTOM-TITLE")
	require.NotContains(t, string(buf), "Whatever")
}

// =============================================================================
// Hot reload via FileChanged
// =============================================================================

func TestFileChangedContentRemove(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Hello")},
		"index.tpl":       {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}}
	ve.templates = map[string]*HtmlTemplate{}

	app := &App{
		contentViews: map[string]*ContentView{
			"GET /post": {Slug: "post", Title: "Hello"},
		},
	}

	err := ve.FileChanged(fsys, app, fsnotifyRemoveEvent("content/post.md"))
	require.NoError(t, err)
	require.Empty(t, app.contentViews)
}

func TestFileChangedContentWrite(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Updated Title")},
		"index.tpl":       {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	app := &App{
		mux:          mux,
		routes:       map[string]*Routing{},
		viewers:      map[string]Viewer{},
		contentViews: map[string]*ContentView{},
		funcMap:      template.FuncMap{},
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}, app: app}
	ve.templates = map[string]*HtmlTemplate{}
	ve.md = newContentRenderer()

	err := ve.FileChanged(fsys, app, fsnotifyWriteEvent("content/post.md"))
	require.NoError(t, err)
	require.NotEmpty(t, app.contentViews)
	require.Equal(t, "Updated Title", app.contentViews["GET /content/post"].Title)
}

func TestFileChangedContentOutsideDirIgnored(t *testing.T) {
	ve := &HtmlViewEngine{contentDirs: []string{"content"}}
	ve.templates = map[string]*HtmlTemplate{}
	app := &App{contentViews: map[string]*ContentView{}}

	err := ve.FileChanged(nil, app, fsnotifyWriteEvent("docs/post.md"))
	require.NoError(t, err)
	require.Empty(t, app.contentViews)
}

// .tpl Write on an already-loaded template must ReLoad in place so the
// existing HtmlViewer (referencing the same *HtmlTemplate) sees the
// change, instead of being shadowed by a freshly-allocated instance.
func TestFileChangedTplWriteReloadsInPlace(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {
			Data: []byte(`<!--layout:site-->
{{define "content"}}<p>v1</p>{{end}}`),
		},
		"content/blog/post.md": {Data: []byte("# Post")},
	}

	mux := http.NewServeMux()
	app := &App{
		mux:          mux,
		routes:       map[string]*Routing{},
		viewers:      map[string]Viewer{},
		contentViews: map[string]*ContentView{},
		funcMap:      template.FuncMap{},
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}, app: app}
	ve.templates = map[string]*HtmlTemplate{}
	ve.md = newContentRenderer()
	ve.Load(fsys, app)

	// Capture the exact *HtmlTemplate instance that the route uses.
	original, ok := ve.templates["content/blog/index.tpl"]
	require.True(t, ok)
	post, ok := app.routes["GET /content/blog/post"]
	require.True(t, ok)
	viewer, ok := post.Viewers[0].(*HtmlViewer)
	require.True(t, ok)
	require.Same(t, original, viewer.template,
		"route viewer must reference the cached *HtmlTemplate")

	// Mutate the file on disk and fire Write.
	fsys["content/blog/index.tpl"] = &fstest.MapFile{
		Data: []byte(`<!--layout:site-->
{{define "content"}}<p>v2</p>{{end}}`),
	}
	require.NoError(t, ve.FileChanged(fsys, app, fsnotifyWriteEvent("content/blog/index.tpl")))

	// Same instance, new content.
	require.Same(t, original, ve.templates["content/blog/index.tpl"],
		"Write must Reload in place, not allocate a new instance")
	require.Same(t, original, viewer.template,
		"existing HtmlViewer must keep seeing the same *HtmlTemplate")

	buf := &bytes.Buffer{}
	require.NoError(t, viewer.template.Execute(buf, ViewModel{Content: &ContentView{Title: "Post"}}))
	require.Contains(t, buf.String(), "v2",
		"post route must render the updated template")
}

// .tpl Remove must drop the cached entry so subsequent loadContentFile
// calls do not point at a stale template.
func TestFileChangedTplRemoveClearsCache(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {
			Data: []byte(`<!--layout:site-->
{{define "content"}}<p>v1</p>{{end}}`),
		},
	}
	app := &App{
		mux:          http.NewServeMux(),
		routes:       map[string]*Routing{},
		viewers:      map[string]Viewer{},
		contentViews: map[string]*ContentView{},
		funcMap:      template.FuncMap{},
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}, app: app}
	ve.templates = map[string]*HtmlTemplate{}
	ve.md = newContentRenderer()
	ve.Load(fsys, app)

	require.Contains(t, ve.templates, "content/blog/index.tpl")

	require.NoError(t, ve.FileChanged(fsys, app, fsnotifyRemoveEvent("content/blog/index.tpl")))
	require.NotContains(t, ve.templates, "content/blog/index.tpl",
		"Remove must drop the cached template")
}

// .tpl Create on a path that has no cached entry must load fresh.
func TestFileChangedTplCreateLoads(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/blog/index.tpl": {
			Data: []byte(`<!--layout:site-->
{{define "content"}}<p>fresh</p>{{end}}`),
		},
	}
	app := &App{
		mux:          http.NewServeMux(),
		routes:       map[string]*Routing{},
		viewers:      map[string]Viewer{},
		contentViews: map[string]*ContentView{},
		funcMap:      template.FuncMap{},
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDirs: []string{"content"}, app: app}
	ve.templates = map[string]*HtmlTemplate{}
	ve.md = newContentRenderer()
	// Skip Load — simulate a runtime that has no template yet.
	require.NotContains(t, ve.templates, "content/blog/index.tpl")

	require.NoError(t, ve.FileChanged(fsys, app, fsnotifyEvent("content/blog/index.tpl", fsnotify.Create)))
	require.Contains(t, ve.templates, "content/blog/index.tpl",
		"Create on missing entry must load the template")
}

// =============================================================================
// helpers
// =============================================================================

// fakeFileInfo is a minimal fs.FileInfo used in tests.
type fakeFileInfo struct {
	name    string
	modTime time.Time
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return 0 }
func (f *fakeFileInfo) Mode() fs.FileMode  { return 0644 }
func (f *fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f *fakeFileInfo) IsDir() bool        { return false }
func (f *fakeFileInfo) Sys() any           { return nil }

func fsnotifyRemoveEvent(name string) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: fsnotify.Remove}
}

func fsnotifyWriteEvent(name string) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: fsnotify.Write}
}

func fsnotifyEvent(name string, op fsnotify.Op) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: op}
}

// Avoid an "imported and not used" warning if any helper above becomes unused.
var _ = strings.Contains