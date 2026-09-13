package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yaitoo/xun/fsnotify"
)

// plainTemplate is the canonical user-facing template: every URL entry
// emitted by App.SitemapURLs becomes one <url> element.
const plainSitemapTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
{{ range .Data -}}
  <url>
    <loc>{{ .Loc }}</loc>
    <lastmod>{{ .LastMod }}</lastmod>
  </url>
{{ end -}}
</urlset>`

// indexTpl is the catch-all bubble-up template used by every test fsys
// that holds .md files under content/. Without it, loadContentFile warns
// "no bubble-up template" and the route never registers — which makes
// SitemapURLs skip the orphan entry, leaving the sitemap empty.
const indexTpl = `{{ define "layout" -}}
<!doctype html><html><body>{{ template "content" . }}</body></html>
{{- end }}`

func TestSitemap_RendersFromContent(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl": {Data: []byte(indexTpl)},
		"content/post.md": &fstest.MapFile{
			Data:    []byte("# Hello"),
			ModTime: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		"content/older.md": &fstest.MapFile{
			Data:    []byte("# Older"),
			ModTime: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// .xml is served with text/xml or application/xml depending on the
	// platform mime database. Both are valid; charset is always utf-8.
	ct := resp.Header.Get("Content-Type")
	require.True(t,
		strings.HasPrefix(ct, "text/xml") || strings.HasPrefix(ct, "application/xml"),
		"unexpected Content-Type %q", ct)
	require.Contains(t, ct, "charset=utf-8")

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(buf)

	// Both posts present, with the test server's host:port and the
	// content-dir prefix that the route actually carries.
	require.Contains(t, body, "<loc>/content/older</loc>")
	require.Contains(t, body, "<loc>/content/post</loc>")
	require.Contains(t, body, "<lastmod>2025-12-01T00:00:00Z</lastmod>")
	require.Contains(t, body, "<lastmod>2026-09-01T00:00:00Z</lastmod>")

	// Sorted by Loc
	olderIdx := strings.Index(body, "/older")
	postIdx := strings.Index(body, "/post")
	require.Less(t, olderIdx, postIdx, "expected older before post in stable sort order")
}

func TestSitemap_OrphanContentViewIsSkipped(t *testing.T) {
	// Regression for review finding #2: loadContentFile writes the .md's
	// ContentView into app.contentViews BEFORE checking for a bubble-up
	// template. Without a template, no route is registered, but the
	// ContentView persists — and would emit a sitemap URL that 404s.
	//
	// Setup: only routed.md has a sibling .tpl; orphan.md has neither
	// a sibling .tpl nor any ancestor index.tpl in the fsys. The fsys
	// intentionally has no root index.tpl so the bubble-up lookup fails
	// for orphan.md.
	fsys := fstest.MapFS{
		"content/routed.md":  {Data: []byte("# R")},
		"content/routed.tpl": {Data: []byte(`{{ define "layout" }}R{{ end }}`)},
		"content/orphan.md":  {Data: []byte("# O")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	// Sanity check: both contentViews entries exist (orphan is still
	// stored — it's used by breadcrumb / .Data.Content — even without a
	// route).
	_, hasRouted := app.contentViews["GET /content/routed"]
	_, hasOrphan := app.contentViews["GET /content/orphan"]
	require.True(t, hasRouted)
	require.True(t, hasOrphan, "orphan contentView should still be present (used by breadcrumb etc.)")

	// Only routed got a route.
	_, routedRoute := app.routes["GET /content/routed"]
	_, orphanRoute := app.routes["GET /content/orphan"]
	require.True(t, routedRoute)
	require.False(t, orphanRoute, "orphan should have no registered route")

	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	require.Contains(t, body, "<loc>/content/routed</loc>")
	require.NotContains(t, body, "<loc>/content/orphan</loc>")
}

func TestSitemap_CustomContentDir_PathIncludesPrefix(t *testing.T) {
	// Regression for review finding #1: cv.Slug strips the content-dir
	// prefix (e.g. "post" for blog/post.md), but the route URL keeps it
	// (e.g. /blog/post). SitemapURLs must derive the URL from the route
	// pattern, not from cv.Slug — otherwise every loc 404s.
	fsys := fstest.MapFS{
		"index.tpl":    {Data: []byte(indexTpl)},
		"blog/post.md": {Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog"))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	// The URL must include the /blog/ prefix, not just /post.
	require.Contains(t, body, "<loc>/blog/post</loc>")
	require.NotContains(t, body, "<loc>/post</loc>")
}

func TestSitemap_IndexPageHasTrailingSlash(t *testing.T) {
	// Regression for review finding #5: blog/index.md → pattern
	// "GET /blog/{$}" → sitemap must emit /blog/ (canonical, matches the
	// route), not /blog (which would 307-redirect and waste a hop on
	// every crawler fetch).
	fsys := fstest.MapFS{
		"index.tpl":       {Data: []byte(indexTpl)},
		"blog/index.md":   {Data: []byte("# Blog Index")},
		"blog/index.tpl":  {Data: []byte(`{{ define "layout" }}B{{ end }}`)},
		"blog/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithContent("blog"))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	require.Contains(t, body, "<loc>/blog/</loc>")
	require.Contains(t, body, "<loc>/blog/post</loc>")
	require.NotContains(t, body, "<loc>/blog</loc>")
}

func TestSitemap_RemovePreservesUserHandler(t *testing.T) {
	// Regression for review finding #2: removing public/sitemap.xml
	// must not clobber a handler the user registered via app.Get.
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())

	customCalled := false
	app.Get("/sitemap.xml", func(c *Context) error {
		customCalled = true
		c.WriteHeader("X-Custom", "yes")
		c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err := c.Response.Write([]byte("custom"))
		return err
	})

	app.Start()
	defer app.Close()

	// Sanity: custom handler runs initially.
	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.True(t, customCalled)
	require.Equal(t, "custom", string(buf))

	// Now simulate the user deleting public/sitemap.xml.
	customCalled = false
	delete(fsys, "public/sitemap.xml")
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Remove})

	// Custom handler must still run.
	req, _ = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t, customCalled)
	buf, _ = io.ReadAll(resp.Body)
	require.Equal(t, "custom", string(buf))
	require.Equal(t, "yes", resp.Header.Get("X-Custom"))
}

func TestSitemap_RecoversFromParseFailure(t *testing.T) {
	// Regression for review finding #3: if the initial template is
	// malformed, the route is served by FileViewer (raw bytes). When
	// the user fixes the template and a Write event fires, the dynamic
	// handler must take over — not the static bytes.
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte("{{ unterminated"), ModTime: time.Now()},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())
	app.Start()
	defer app.Close()

	// Initial: FileViewer fallback, raw bytes.
	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, "{{ unterminated", string(buf))

	// User fixes the file.
	fsys["public/sitemap.xml"] = &fstest.MapFile{
		Data: []byte(plainSitemapTemplate),
		ModTime: time.Now(),
	}
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Write})

	// Now the dynamic handler must serve the URL list.
	req, _ = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf, _ = io.ReadAll(resp.Body)
	body := string(buf)
	require.Contains(t, body, "<loc>/content/post</loc>")
	require.NotEqual(t, "{{ unterminated", string(buf))
}

func TestSitemap_RemoveAndRecreate_ReinstallsHandler(t *testing.T) {
	// Regression for review finding #4: removing then recreating
	// public/sitemap.xml must reinstall the dynamic handler. The
	// auto-handler closure does a runtime viewer lookup, so after
	// recreation the new viewer is picked up automatically.
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    {Data: []byte("# P")},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate), ModTime: time.Now()},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())
	app.Start()
	defer app.Close()

	// Remove → 404.
	delete(fsys, "public/sitemap.xml")
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Remove})

	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Recreate → dynamic handler serves URLs again.
	fsys["public/sitemap.xml"] = &fstest.MapFile{
		Data:    []byte(plainSitemapTemplate),
		ModTime: time.Now(),
	}
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Create})

	req, _ = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, body, "<loc>/content/post</loc>")
}

func TestSitemap_ViewerExposedAndUsable(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md":     &fstest.MapFile{Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	// The viewer must be exposed under the route's logical name, mirroring
	// StaticViewEngine's "sitemap.xml" route URL after stripping public/.
	v, ok := app.viewers[sitemapName]
	require.True(t, ok, "expected app.viewers[%q] to be populated", sitemapName)
	_, isText := v.(*TextViewer)
	require.True(t, isText, "expected app.viewers[%q] to be *TextViewer", sitemapName)
}

func TestSitemap_UserHandlerOverridesAutoRegistration(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl":          {Data: []byte(indexTpl)},
		"content/post.md":    &fstest.MapFile{Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	// User takes over the route BEFORE engines load. handleSitemap must
	// yield to this registration but still expose the viewer.
	app.Get("/sitemap.xml", func(c *Context) error {
		return c.View(c.App.SitemapURLs(), sitemapName)
	})

	app.Start()
	defer app.Close()

	// Viewer is still exposed for the user's handler to reuse.
	_, ok := app.viewers[sitemapName]
	require.True(t, ok)

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(buf), "<loc>/content/post</loc>")
}

func TestSitemap_NoFile_404FromMux(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": &fstest.MapFile{Data: []byte("# P")},
		// no public/sitemap.xml
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	// No viewer registered: user can't accidentally reach a half-built sitemap.
	_, ok := app.viewers[sitemapName]
	require.False(t, ok)

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSitemap_ParseFailure_FallsBackToFileViewer(t *testing.T) {
	// Malformed template: unterminated action. text/template returns an error.
	fsys := fstest.MapFS{
		"content/post.md":    &fstest.MapFile{Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte("{{ unterminated")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// FileViewer fallback: bytes are served verbatim (no template execution).
	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "{{ unterminated", string(buf))
}

func TestSitemap_FileChanged_Reloads(t *testing.T) {
	fsys := fstest.MapFS{
		"index.tpl": {Data: []byte(indexTpl)},
		"content/post.md": &fstest.MapFile{
			Data:    []byte("# P"),
			ModTime: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		"public/sitemap.xml": &fstest.MapFile{
			Data:    []byte(plainSitemapTemplate),
			ModTime: time.Now(),
		},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())
	app.Start()
	defer app.Close()

	// Initial render.
	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	first := string(buf)
	require.Contains(t, first, "<loc>/content/post</loc>")

	// Mutate the file on disk and deliver a Write event by hand. The poll
	// loop is parked (see TestMain), so we can race neither.
	fsys["public/sitemap.xml"] = &fstest.MapFile{
		Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset>
  <url><loc>HANDOFF</loc><lastmod></lastmod></url>
{{ range .Data -}}
  <url><loc>{{ .Loc }}</loc></url>
{{ end -}}
</urlset>`),
		ModTime: time.Now(),
	}

	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Write})

	req, _ = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	second := string(buf)

	// New template renders the sentinel, confirming the route handler was
	// re-bound to a freshly-parsed template.
	require.Contains(t, second, "HANDOFF")
	require.Contains(t, second, "<loc>/content/post</loc>")
}

func TestSitemap_FileChanged_Remove_ResetsToNotFound(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md":     &fstest.MapFile{Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys), WithWatch())
	app.Start()
	defer app.Close()

	// Initial: 200 with rendered body.
	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Remove from fsys and deliver a Remove event.
	delete(fsys, "public/sitemap.xml")
	reload(app, fsnotify.Event{Name: "public/sitemap.xml", Op: fsnotify.Remove})

	req, _ = http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Viewer removed too.
	_, ok := app.viewers[sitemapName]
	require.False(t, ok)
}

func TestSitemap_PlainXML_NoActions_RendersIdentity(t *testing.T) {
	// Static-looking sitemap (no {{ actions }}) still goes through the
	// template path. text/template treats the entire body as a literal,
	// so output == input bytes (modulo the http.Content-Type header).
	const literal = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/hardcoded</loc>
  </url>
</urlset>`

	fsys := fstest.MapFS{
		"content/post.md":    &fstest.MapFile{Data: []byte("# P")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(literal)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, literal, string(buf))
}

func TestSitemap_OnlyContentRoutes(t *testing.T) {
	// SitemapURLs walks app.contentViews, not app.routes. pages/*.html,
	// public/*.html, and app.Get routes are intentionally outside the
	// framework's sitemap scope — see content_sitemap.go for the
	// rationale. This test pins that scope: only content/*.md entries
	// with a registered route appear.
	fsys := fstest.MapFS{
		"index.tpl":               {Data: []byte(indexTpl)},
		"pages/about.html":        {Data: []byte(`<h1>About</h1>`)},
		"pages/blog/index.html":   {Data: []byte(`<h1>Blog</h1>`)},
		"content/post.md":         {Data: []byte("# P")},
		"public/about.html":       {Data: []byte(`<h1>Static</h1>`)},
		"public/style.css":        {Data: []byte("body{}")},
		"public/sitemap.xml":      {Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	app := New(WithMux(mux), WithFsys(fsys))
	app.Get("/api/health", func(c *Context) error {
		return c.View("ok", "text/plain")
	})
	app.Start()
	defer app.Close()

	urls := app.SitemapURLs()

	locSet := map[string]bool{}
	for _, u := range urls {
		locSet[u.Loc] = true
	}

	require.Contains(t, locSet, "/content/post", "content/post.md is the only source")
	require.NotContains(t, locSet, "/about", "pages/*.html is outside sitemap scope")
	require.NotContains(t, locSet, "/blog/", "pages/*.html is outside sitemap scope")
	require.NotContains(t, locSet, "/about.html", "public/*.html is outside sitemap scope")
	require.NotContains(t, locSet, "/api/health", "user app.Get is outside sitemap scope")
	require.NotContains(t, locSet, "/style.css", "static asset is outside sitemap scope")
}

func TestSitemap_LastModOnlyForContentRoutes(t *testing.T) {
	// LastMod is only meaningful for content engine routes — pages and
	// user handlers don't have a tracked mtime. The template-side guard
	// `{{ if .LastMod }}` relies on this.
	fsys := fstest.MapFS{
		"index.tpl": {Data: []byte(indexTpl)},
		"pages/page.html": {Data: []byte("<h1>P</h1>")},
		"content/post.md": &fstest.MapFile{
			Data:    []byte("# P"),
			ModTime: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		"public/sitemap.xml": {Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	app := New(WithMux(mux), WithFsys(fsys))
	app.Start()
	defer app.Close()

	urls := app.SitemapURLs()

	byLoc := map[string]SitemapURL{}
	for _, u := range urls {
		byLoc[u.Loc] = u
	}

	require.Equal(t, "2026-09-01T00:00:00Z", byLoc["/content/post"].LastMod,
		"content route should carry LastMod from mtime")
	require.Empty(t, byLoc["/page"].LastMod,
		"page route has no mtime; LastMod must be empty for the {{ if .LastMod }} guard")
}