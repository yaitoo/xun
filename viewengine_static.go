package xun

import (
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"reflect"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// StaticViewEngine is a view engine that serves static files from a file system.
type StaticViewEngine struct {
	isEmbedFsys bool
}

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
// directory, nothing should be done — except for public/sitemap.xml and
// public/sitemap-index.xml, whose viewers are re-published on Write/Create
// and dropped on Remove.
func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	if name, ok := sitemapViewerName(event.Name); ok {
		switch {
		case event.Has(fsnotify.Remove):
			// Just clear the viewer. The sitemap route resolves
			// app.viewers[sitemapName] on every request, so a missing
			// viewer turns into a 404 from inside the handler. A user-taken-
			// over route is unaffected — their handler never inspects our
			// viewer map. This avoids clobbering user handlers with a stub
			// notFoundHandler on every dev-only file delete.
			delete(app.viewers, name)
		case event.Has(fsnotify.Write), event.Has(fsnotify.Create):
			ve.handleSitemap(fsys, app, event.Name, name)
		}
		return nil
	}

	// Nothing should be updated for Write/Remove events.
	if strings.HasPrefix(event.Name, "public/") && (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

// sitemapViewerName returns the app.viewers key a sitemap template is
// published under, and whether the path is a sitemap template at all.
//
// The two templates are published as viewers, never as static files: the
// framework's sitemap routes render them with the collected entries.
func sitemapViewerName(path string) (string, bool) {
	switch path {
	case sitemapPath:
		return sitemapName, true
	case sitemapIndexPath:
		return sitemapIndexName, true
	}

	return "", false
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

	// public/sitemap.xml and public/sitemap-index.xml are special cases:
	// parse them as text/templates and expose the resulting viewers under
	// app.viewers. The routes at GET /sitemap.xml and GET /sitemaps/{name}
	// are owned by the framework (see App.Sitemap and App.SitemapIndex) —
	// this engine only publishes viewers.
	if name, ok := sitemapViewerName(path); ok {
		ve.handleSitemap(fsys, app, path, name)
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

// handleSitemap parses path as a TextTemplate and exposes the TextViewer via
// app.viewers[name]. The route serving the template renders that viewer with
// the collected entries as the Data payload.
//
// The engine publishes the viewer only — it never registers a route. The
// route belongs to the framework (see App.Sitemap and App.SitemapIndex) and
// resolves the viewer on every request, so a template that appears, changes
// or disappears at runtime needs no route surgery.
//
// When the template cannot be parsed, a FileViewer serving the raw bytes is
// published under the same name instead: the URL still answers with the
// file's content rather than failing to render.
func (ve *StaticViewEngine) handleSitemap(fsys fs.FS, app *App, path, name string) {
	t := &TextTemplate{name: path}
	if err := t.Load(fsys, app.funcMap); err != nil {
		app.logger.Error("xun: parse sitemap",
			slog.String("path", path), slog.Any("err", err))
		// Parse failure: publish a FileViewer so the URL still serves bytes.
		app.viewers[name] = NewFileViewer(fsys, path, ve.isEmbedFsys, "", "")
		return
	}

	app.viewers[name] = NewTextViewer(t)
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
