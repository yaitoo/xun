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

func TestSitemap_RendersFromContent(t *testing.T) {
	fsys := fstest.MapFS{
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

	// Both posts present, with the test server's host:port
	require.Contains(t, body, "<loc>"+srv.URL+"/older</loc>")
	require.Contains(t, body, "<loc>"+srv.URL+"/post</loc>")
	require.Contains(t, body, "<lastmod>2025-12-01T00:00:00Z</lastmod>")
	require.Contains(t, body, "<lastmod>2026-09-01T00:00:00Z</lastmod>")

	// Sorted by Loc
	olderIdx := strings.Index(body, "/older")
	postIdx := strings.Index(body, "/post")
	require.Less(t, olderIdx, postIdx, "expected older before post in stable sort order")
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
		return c.View(c.App.SitemapURLs(SitemapOptions{}, c), sitemapName)
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
	require.Contains(t, string(buf), "<loc>"+srv.URL+"/post</loc>")
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

func TestSitemap_FilterDropsEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"content/post.md": &fstest.MapFile{
			Data:    []byte("# P"),
			ModTime: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		"content/draft.md": &fstest.MapFile{
			Data:    []byte("# D"),
			ModTime: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		},
		"content/draft.yaml": &fstest.MapFile{Data: []byte("draft: true\n")},
		"public/sitemap.xml": &fstest.MapFile{Data: []byte(plainSitemapTemplate)},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(
		WithMux(mux),
		WithFsys(fsys),
		WithSitemap(Sitemap{
			Filter: func(cv *ContentView) bool {
				if cv.Params == nil {
					return true
				}
				d, _ := cv.Params["draft"].(bool)
				return !d
			},
		}),
	)
	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/sitemap.xml", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(buf)

	require.Contains(t, body, "<loc>"+srv.URL+"/post</loc>")
	require.NotContains(t, body, "<loc>"+srv.URL+"/draft</loc>")
}

func TestSitemap_FileChanged_Reloads(t *testing.T) {
	fsys := fstest.MapFS{
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
	require.Contains(t, first, "<loc>"+srv.URL+"/post</loc>")

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
	require.Contains(t, second, "<loc>"+srv.URL+"/post</loc>")
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