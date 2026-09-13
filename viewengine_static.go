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
// re-parses on Write/Create and clears the viewer on Remove.
func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	if event.Name == sitemapPath {
		switch {
		case event.Has(fsnotify.Remove):
			// Just clear the viewer. The auto-handler closure (if installed)
			// re-reads app.viewers[sitemapName] on every request, so a missing
			// viewer turns into a 404 from inside the closure. A user-taken-
			// over route is unaffected — their handler never inspects our
			// viewer map. This avoids clobbering user handlers with a stub
			// notFoundHandler on every dev-only file delete.
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
// the TextViewer via app.viewers[sitemapName]. The auto-registered route
// handler at GET /sitemap.xml renders the viewer with App.SitemapURLs as
// the Data payload.
//
// Three branches over route state:
//
//   - No route yet → install the auto-handler closure (initial Load or
//     first time the file appears). The closure looks up app.viewers
//     dynamically so later Write events just refresh the viewer.
//
//   - Route exists with a FileViewer viewer (r.Viewers[0] == *FileViewer)
//     → this is the parse-failure fallback from a prior Load. Replace
//     with the auto-handler closure so the dynamic template takes over.
//     This is the recovery path: malformed template on first Load →
//     fix → Write event → parse succeeds → upgrade.
//
//   - Route exists with anything else → user owns it (or our own
//     auto-handler closure from an earlier successful Load). Don't touch
//     r.Handle; the closure will see the refreshed viewer on the next
//     request via the dynamic lookup.
func (ve *StaticViewEngine) handleSitemap(fsys fs.FS, app *App, path string) {
	t := &TextTemplate{name: path}
	if err := t.Load(fsys, app.funcMap); err != nil {
		app.logger.Error("xun: parse sitemap",
			slog.String("path", path), slog.Any("err", err))
		// Parse failure: register FileViewer so the URL still serves bytes.
		// HandleFile is first-writer-wins — if the user already took the
		// route, this is a no-op and the user's handler keeps running.
		app.HandleFile("sitemap.xml", NewFileViewer(fsys, path, ve.isEmbedFsys, "", ""))
		return
	}

	viewer := NewTextViewer(t)
	app.viewers[sitemapName] = viewer

	if r, exists := app.routes[sitemapKey]; exists {
		// Recovery: a previous parse failure left a FileViewer at this
		// route. Replace it with the dynamic template handler.
		if len(r.Viewers) > 0 {
			if _, isFile := r.Viewers[0].(*FileViewer); isFile {
				app.createHandler(sitemapKey, sitemapHandler, nil, app)
			}
		}
		// Otherwise: user owns it, or our auto-handler already runs and
		// will see the refreshed viewer on the next request.
		return
	}

	app.createHandler(sitemapKey, sitemapHandler, nil, app)
}

// sitemapHandler is the auto-registered handler for GET /sitemap.xml.
// Resolves the viewer dynamically on each request so FileChanged Write
// events can refresh the viewer without rebuilding the closure. When the
// viewer is absent (e.g., right after a Remove event), it returns 404 —
// we never touch r.Handle on Remove, so we don't risk clobbering a
// user-taken-over route.
func sitemapHandler(c *Context) error {
	v, ok := c.App.viewers[sitemapName]
	if !ok {
		c.WriteStatus(http.StatusNotFound)
		return nil
	}
	return v.Render(c, c.App.SitemapURLs())
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
