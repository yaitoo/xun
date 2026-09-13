package xun

import (
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// StaticViewEngine is a view engine that serves static files from a file system.
type StaticViewEngine struct {
	isEmbedFsys bool
}

// Sitemap convention: a public/sitemap.xml file is parsed as a text/template
// (via TextTemplate) and rendered through a TextViewer. The viewer is always
// exposed as app.viewers["sitemap.xml"] so user handlers can call
// c.View(data, "sitemap.xml") to reuse it — the viewer name matches the
// route URL that StaticViewEngine registers after stripping the public/
// prefix. The route handler at GET /sitemap.xml is only auto-registered
// when the user has not already taken over that key with app.Get / HandlePage.
const (
	sitemapPath = "public/sitemap.xml" // fsys path; kept verbatim for fs.ReadFile/Stat
	sitemapKey  = "GET /sitemap.xml"   // routes-map key (after splitFile strips public/)
	sitemapName = "sitemap.xml"        // app.viewers key; matches the route path
)

// Load loads all static files from the given file system and registers them with the application.
//
// It scans the "public" directory in the given file system and registers each file
// with the application. It also handles file changes for the "public" directory
// and updates the application accordingly.
func (ve *StaticViewEngine) Load(fsys fs.FS, app *App) {
	root, err := fsys.Open(".")
	if err == nil {
		t := reflect.TypeOf(root)
		if t.Kind() == reflect.Ptr { //nolint: govet
			ve.isEmbedFsys = t.Elem().PkgPath() == "embed"
		}
		root.Close() // nolint: errcheck
	}

	fs.WalkDir(fsys, "public", func(path string, d fs.DirEntry, err error) error { // nolint: errcheck
		if d != nil && !d.IsDir() {
			ve.handle(fsys, app, path)
		}

		return nil
	})
}

// FileChanged handles file changes for the given file system and updates the
// application accordingly. It is called by the watcher when a file is changed.
//
// If the file changed is a Create event and the path is in the "public" directory,
// it will be registered with the application.
//
// If the file changed is a Write/Remove event and the path is in the "public"
// directory, nothing should be done — except for public/sitemap.xml, which
// re-parses on Write/Create and resets the route to 404 on Remove.
func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	if event.Name == sitemapPath {
		switch {
		case event.Has(fsnotify.Remove):
			// Replace the handler with a 404 instead of delete(app.routes, ...):
			// http.ServeMux does not support unregistering patterns, and the
			// closure captured by mux holds `r` by pointer — deleting the
			// routes-map entry would still leave the mux closure live, so a
			// subsequent createHandler on the same pattern would panic on
			// duplicate registration. The residual routes-map entry is a
			// debuggability nit (app.Routes() still lists GET /sitemap.xml),
			// but the served behavior is correct: notFoundHandler runs.
			if r, ok := app.routes[sitemapKey]; ok {
				r.Handle = notFoundHandler
				r.Viewers = nil
			}
			delete(app.viewers, sitemapName)
		case event.Has(fsnotify.Write), event.Has(fsnotify.Create):
			ve.handleSitemap(fsys, app, event.Name)
		}
		return nil
	}

	// Nothing should be updated for Write/Remove events.
	if strings.HasPrefix(event.Name, "public/") && (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

	// public/sitemap.xml is a special case: parse it as a text/template and
	// expose the resulting viewer under app.viewers[sitemapPath]. The route
	// handler at GET /sitemap.xml is auto-registered only when the user has
	// not already taken over that route key.
	if path == sitemapPath {
		ve.handleSitemap(fsys, app, path)
		return
	}

	pattern := path

	if strings.HasSuffix(pattern, "/index.html") { // remove it, because index.html will be redirected to ./ in http.ServeFileFS
		pattern = pattern[:len(pattern)-10]
	}

	pattern = strings.TrimPrefix(pattern, "public/")

	app.HandleFile(pattern, NewFileViewer(fsys, path, ve.isEmbedFsys, "", ""))

	for _, m := range app.buildAssetURLs {
		if m("/" + pattern) {
			ve.handleAssetUrl(fsys, app, path, pattern)
			break
		}
	}
}

// handleSitemap parses public/sitemap.xml as a TextTemplate and exposes
// the TextViewer via app.viewers[sitemapPath]. The auto-registered route
// handler at GET /sitemap.xml renders the viewer with App.SitemapURLs as
// the Data payload.
//
// Override semantics:
//   - parse failure → fall back to FileViewer (preserves existing behavior
//     for malformed template content; the user still gets bytes for the URL).
//   - app.routes[sitemapKey] already exists → skip auto-registration; the
//     user owns the route. app.viewers[sitemapPath] is still populated so
//     the user's handler can call c.View(data, "public/sitemap.xml") to
//     reuse the parsed viewer.
func (ve *StaticViewEngine) handleSitemap(fsys fs.FS, app *App, path string) {
	t := &TextTemplate{name: path}
	if err := t.Load(fsys, app.funcMap); err != nil {
		app.logger.Error("xun: parse sitemap",
			slog.String("path", path), slog.Any("err", err))
		// FileViewer 兜底：HandleFile 是 first-writer-wins，路由已被用户接管则跳过
		app.HandleFile("sitemap.xml", NewFileViewer(fsys, path, ve.isEmbedFsys, "", ""))
		return
	}

	viewer := NewTextViewer(t)
	// 始终暴露 viewer：用户 handler 可通过 c.View(data, "sitemap.xml") 复用
	app.viewers[sitemapName] = viewer
	// 仅当 route 未被用户接管时才自动注册 handler。
	// 闭包动态查 viewer：FileChanged Write 只需更新 app.viewers[sitemapName]，
	// 下次请求闭包自然拿到新 TextViewer，无需重建 r.Handle。
	if _, exists := app.routes[sitemapKey]; !exists {
		app.createHandler(sitemapKey, func(c *Context) error {
			v, ok := c.App.viewers[sitemapName]
			if !ok {
				c.WriteStatus(http.StatusNotFound)
				return nil
			}
			return v.Render(c, c.App.SitemapURLs(SitemapOptions{
				Filter: c.App.sitemapFilter,
			}, c))
		}, nil, app)
	}
}

// notFoundHandler is installed at GET /sitemap.xml when public/sitemap.xml
// is removed at runtime, so in-flight requests don't crash on a nil Handle.
func notFoundHandler(c *Context) error {
	c.WriteStatus(http.StatusNotFound)
	return nil
}

const cacheControl = "public, max-age=31536000, immutable"

func (ve *StaticViewEngine) handleAssetUrl(fsys fs.FS, app *App, fileName, pattern string) {
	f, _ := fsys.Open(fileName) // nolint: errcheck
	defer f.Close()            // nolint: errcheck

	buf, _ := io.ReadAll(f) // nolint: errcheck
	etag := ComputeETag(bytes.NewReader(buf))
	ext := path.Ext(pattern)

	assetURL := strings.TrimRight(pattern, ext) + "-" + strings.Trim(etag, "\"") + ext

	app.HandleFile(assetURL,
		NewFileViewer(fsys, fileName, ve.isEmbedFsys, etag, cacheControl))

	app.AssetURLs["/"+pattern] = "/" + assetURL
}
