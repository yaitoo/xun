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
		if t.Kind() == reflect.Ptr {
			ve.isEmbedFsys = t.Elem().PkgPath() == "embed"
		}
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
// directory, nothing will be done.
func (ve *StaticViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
	// Nothing should be updated for Write/Remove events.
	if strings.HasPrefix(event.Name, "public/") && (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) {
		ve.handle(fsys, app, event.Name)
	}

	return nil
}

func (ve *StaticViewEngine) handle(fsys fs.FS, app *App, path string) {

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

const cacheControl = "public, max-age=31536000, immutable"

func (ve *StaticViewEngine) handleAssetUrl(fsys fs.FS, app *App, fileName, pattern string) {

	f, err := fsys.Open(fileName)
	if err != nil {
		app.logger.Error("xun: handleAssetUrl", slog.String("file", fileName), slog.Any("err", err))
		return
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		app.logger.Error("xun: handleAssetUrl", slog.String("file", fileName), slog.Any("err", err))
		return
	}

	etag := ComputeETag(bytes.NewReader(buf))

	ext := path.Ext(pattern)

	assetURL := strings.TrimRight(pattern, ext) + "-" + strings.Trim(etag, "\"") + ext

	app.HandleFile(assetURL,
		NewFileViewer(fsys, fileName, ve.isEmbedFsys, etag, cacheControl))

	app.AssetURLs["/"+pattern] = "/" + assetURL
}
