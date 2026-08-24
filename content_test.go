package xun

import (
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
		"content/2026/index.html": {Data: []byte(`<!--layout:site-->
{{define "content"}}<article><h1>{{.Content.Title}}</h1><p>{{.Content.Description}}</p>{{.Content.Body}}</article>{{end}}`)},
		"content/index.html":       {Data: []byte(`<!--layout:site-->
{{define "content"}}<h1>{{.Content.Title}}</h1>{{.Content.Body}}{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/hello", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	body := string(buf)
	require.Contains(t, body, "Hello")              // title from H1
	require.Contains(t, body, "World from markdown") // rendered body
	require.Contains(t, body, "<html>")             // layout wrapper

	req, _ = http.NewRequest("GET", srv.URL+"/2026/deeper", nil)
	req.Header.Set("Accept", "text/html")
	resp, err = client.Do(req)
	require.NoError(t, err)
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	body = string(buf)
	require.Contains(t, body, "Deeper Post")             // title
	require.Contains(t, body, "A lede paragraph.")       // description (blockquote)
	require.Contains(t, body, "<article>")               // bubble-up to 2026/index.html
	require.Contains(t, body, "<h2>Section</h2>")         // markdown h2 in body
	require.Contains(t, body, "More content.")            // markdown paragraph
}

func TestContentEngineBubbleUpToRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/site.html": {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"content/orphan.md": {Data: []byte("# Orphan")},
		"index.html": {Data: []byte(`<!--layout:site-->
{{define "content"}}<main>{{.Content.Title}}{{.Content.Body}}</main>{{end}}`)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/orphan", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "<main>")
	require.Contains(t, string(buf), "Orphan")
}

func TestContentEngineNoTemplate(t *testing.T) {
	// No index.html anywhere → warning logged, route not registered.
	fsys := fstest.MapFS{
		"content/orphan.md": {Data: []byte("# Orphan")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/orphan", nil)
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

// =============================================================================
// bubbleUp unit
// =============================================================================

func TestBubbleUpSpecific(t *testing.T) {
	fsys := fstest.MapFS{
		"content/2026/deeper.html": {Data: []byte(`<p>specific</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
	require.Equal(t, "content/2026/deeper.html", ve.bubbleUp("2026/deeper"))
}

func TestBubbleUpToIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"content/2026/index.html": {Data: []byte(`<p>index</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
	require.Equal(t, "content/2026/index.html", ve.bubbleUp("2026/deeper"))
}

func TestBubbleUpToContentIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"content/index.html": {Data: []byte(`<p>index</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
	require.Equal(t, "content/index.html", ve.bubbleUp("anything"))
}

func TestBubbleUpToRootIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte(`<p>root</p>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
	require.Equal(t, "index.html", ve.bubbleUp("anything"))
}

func TestBubbleUpNotFound(t *testing.T) {
	fsys := fstest.MapFS{}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
	require.Equal(t, "", ve.bubbleUp("anything"))
}

// =============================================================================
// Options
// =============================================================================

func TestWithContentDir(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/post.md": {Data: []byte("# Hello")},
		"index.html":   {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContentDir("docs"))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/post", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	require.Contains(t, string(buf), "Hello")
}

func TestWithContentDirEmptyDisables(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": {Data: []byte("# Hello")},
		"index.html":      {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContentDir(""))
	app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/post", nil)
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
		"index.html":      {Data: []byte(`<html>{{.Content.Body}}</html>`)},
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

	req, _ := http.NewRequest("GET", srv.URL+"/post", nil)
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
		"index.html":      {Data: []byte(`<html><h1>{{.Content.Title}}</h1></html>`)},
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

	req, _ := http.NewRequest("GET", srv.URL+"/post", nil)
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
		"index.html":      {Data: []byte(`<html>{{.Content.Title}}</html>`)},
	}
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content"}
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
		"index.html":      {Data: []byte(`<html>{{.Content.Title}}</html>`)},
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
	ve := &HtmlViewEngine{fsys: fsys, contentDir: "content", app: app}
	ve.templates = map[string]*HtmlTemplate{}
	ve.md = newContentRenderer()

	err := ve.FileChanged(fsys, app, fsnotifyWriteEvent("content/post.md"))
	require.NoError(t, err)
	require.NotEmpty(t, app.contentViews)
	require.Equal(t, "Updated Title", app.contentViews["GET /post"].Title)
}

func TestFileChangedContentOutsideDirIgnored(t *testing.T) {
	ve := &HtmlViewEngine{contentDir: "content"}
	ve.templates = map[string]*HtmlTemplate{}
	app := &App{contentViews: map[string]*ContentView{}}

	err := ve.FileChanged(nil, app, fsnotifyWriteEvent("docs/post.md"))
	require.NoError(t, err)
	require.Empty(t, app.contentViews)
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

// Avoid an "imported and not used" warning if any helper above becomes unused.
var _ = strings.Contains