package xun

import (
	"html/template"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	"github.com/yaitoo/xun/fsnotify"
)

// HtmlViewEngine is a view engine that loads templates from a file system.
//
// It supports 2 types of templates:
//   - Components: These are templates that are loaded from the "components" directory.
//   - Pages: These are templates that are loaded from the "layouts/views/pages/" directory.
//
// Components are used to build up larger templates, while pages are used to render
// the final HTML that is sent to the client.
type HtmlViewEngine struct {
	fsys fs.FS
	app  *App

	templates map[string]*HtmlTemplate

	// Content engine: loads .md files from one or more content directories,
	// renders them to HTML, and registers routes via bubble-up template lookup.
	// Each directory becomes a URL prefix (e.g. blog/post.md → /blog/post).
	md            *contentRenderer
	contentDirs   []string                                      // default ["content"]; empty disables
	metaExtractor func(string, []byte, fs.FileInfo) ContentView // nil → use extractContentView
	renderFn      func([]byte, string) (template.HTML, error)   // nil → use md.Render
}

// Load loads all templates from the given file system.
//
// It loads all components, layouts, pages and views from the given file system.
func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) {
	if ve.templates == nil {
		ve.templates = map[string]*HtmlTemplate{}
	}

	ve.fsys = fsys
	ve.app = app

	if ve.md == nil {
		ve.md = newContentRenderer()
	}

	ve.loadComponents()
	ve.loadLayouts()
	ve.loadPages()
	ve.loadViews()
	ve.loadContent()
}

// FileChanged is called when a file has been changed.
//
// It is used to reload templates when they have been changed.
func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error { // skipcq: RVV-B0012

	ext := strings.ToLower(filepath.Ext(event.Name))

	switch ext {
	case ".html":
		if event.Has(fsnotify.Remove) {
			return nil // route de-registration not supported (same as before)
		}
		name := event.Name[:len(event.Name)-len(".html")]
		if event.Has(fsnotify.Write) {
			if t, ok := ve.templates[name]; ok {
				return t.Reload(fsys, ve.templates, app.funcMap)
			}
			return nil
		}
		if !event.Has(fsnotify.Create) {
			return nil
		}
		switch {
		case strings.HasPrefix(event.Name, "components/"),
			strings.HasPrefix(event.Name, "layouts/"):
			_, err := ve.loadTemplate(event.Name)
			return err
		case strings.HasPrefix(event.Name, "pages/"):
			return ve.loadPage(event.Name)
		case strings.HasPrefix(event.Name, "views/"):
			return ve.loadView(event.Name)
		}
		if dir, ok := ve.matchedContentDir(event.Name); ok {
			return ve.loadContentPage(event.Name, dir)
		}
		return nil

	case ".md":
		dir, ok := ve.matchedContentDir(event.Name)
		if !ok {
			return nil
		}
		if event.Has(fsnotify.Remove) {
			slug := strings.TrimSuffix(strings.TrimPrefix(event.Name, dir+"/"), ".md")
			app.mu.Lock()
			delete(app.contentViews, "GET /"+slug)
			app.mu.Unlock()
			return nil
		}
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			return ve.loadContentFile(event.Name, dir)
		}
		return nil

	case ".tpl":
		dir, ok := ve.matchedContentDir(event.Name)
		if !ok {
			return nil
		}
		tmplKey := ve.templateKey(event.Name)
		if event.Has(fsnotify.Remove) {
			// No route to de-register (templates never register routes).
			// Drop the cached template so a later loadContentFile with
			// the same path rebuilds it from disk.
			delete(ve.templates, tmplKey)
			return nil
		}
		// Write/Create: prefer Reload so the existing HtmlTemplate
		// instance stays in place — HtmlViewer entries in app.routes
		// hold a pointer to that instance, and replacing it would leave
		// them rendering the stale template. Reload also cascades to
		// dependents via the dependency graph.
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			if t, ok := ve.templates[tmplKey]; ok {
				return t.Reload(fsys, ve.templates, app.funcMap)
			}
			_, err := ve.loadContentTemplate(event.Name, dir)
			return err
		}
		return nil
	}

	return nil
}

func (ve *HtmlViewEngine) loadFiles(dir string, process func(path string) error) {
	fs.WalkDir(ve.fsys, dir, func(path string, d fs.DirEntry, _ error) error { // nolint: errcheck
		if !strings.EqualFold(filepath.Ext(path), ".html") {
			return nil
		}
		if err := process(path); err != nil {
			ve.app.logger.Error("xun: load html", slog.String("path", path), slog.Any("err", err))
		}
		return nil
	})
}

func (ve *HtmlViewEngine) loadComponents() {
	ve.loadFiles("components", func(path string) error {
		_, err := ve.loadTemplate(path)
		return err
	})
}

func (ve *HtmlViewEngine) loadTemplate(path string) (*HtmlTemplate, error) {
	name := path[:len(path)-5]

	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
		return nil, err
	}

	for n := range t.dependencies {
		d, ok := ve.templates[n]
		if ok {
			d.dependents[name] = t
		}
	}

	ve.templates[name] = t

	return t, nil
}

func (ve *HtmlViewEngine) loadLayouts() {
	ve.loadFiles("layouts", func(path string) error {
		_, err := ve.loadTemplate(path)
		return err
	})
}

func (ve *HtmlViewEngine) loadPages() {
	ve.loadFiles("pages", func(path string) error { // nolint: errcheck
		return ve.loadPage(path)
	})
}

func (ve *HtmlViewEngine) loadPage(path string) error {
	name := path[6:] // delete prefix  "pages/"

	t := NewHtmlTemplate(name, path)

	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
		return err
	}

	// delete file extension ".html"
	ve.templates[path[:len(path)-5]] = t

	if strings.HasSuffix(path, "/index.html") { // remove it, because index.html will be redirected to ./ in http.ServeFileFS
		name = name[:len(name)-10]
	}

	_, _, pattern := splitFile(name)
	pattern = strings.TrimSuffix(pattern, ".html")

	ve.app.HandlePage(pattern, path[6:len(path)-5], &HtmlViewer{
		template: t,
	})

	return nil
}

func (ve *HtmlViewEngine) loadViews() {
	ve.loadFiles("views", func(path string) error {
		return ve.loadView(path)
	})
}

func (ve *HtmlViewEngine) loadView(path string) error {

	t, err := ve.loadTemplate(path)
	if err != nil {
		return err
	}

	ve.app.viewers[path[:len(path)-5]] = &HtmlViewer{
		template: t,
	}

	return nil
}

// loadContent walks every configured content directory and dispatches by
// file extension:
//   - .md   → loadContentFile       (parse, render, register route via bubble-up)
//   - .html → loadContentPage       (register as a regular page route)
//   - .tpl  → loadContentTemplate   (load as a bubble-up template; no route)
//
// Subdirectories are walked recursively. Each content directory becomes
// a URL prefix; e.g. blog/post.md → /blog/post and docs/api/intro.md →
// /docs/api/intro.
//
// The split between .html and .tpl is what makes directory-level pages
// possible: a directory may carry an index.tpl (template) and an
// index.md (real page at /<dir>/) without the template occupying the route.
func (ve *HtmlViewEngine) loadContent() {
	for _, dir := range ve.contentDirs {
		if dir == "" {
			continue
		}
		// nolint:errcheck
		fs.WalkDir(ve.fsys, dir, func(p string, d fs.DirEntry, _ error) error {
			if d == nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			switch ext {
			case ".md":
				if err := ve.loadContentFile(p, dir); err != nil {
					ve.app.logger.Error("xun: load content", slog.String("path", p), slog.Any("err", err))
				}
			case ".html":
				if err := ve.loadContentPage(p, dir); err != nil {
					ve.app.logger.Error("xun: load content page", slog.String("path", p), slog.Any("err", err))
				}
			case ".tpl":
				if _, err := ve.loadContentTemplate(p, dir); err != nil {
					ve.app.logger.Error("xun: load content template", slog.String("path", p), slog.Any("err", err))
				}
			}
			return nil
		})
	}
}

// matchedContentDir returns the longest configured content directory that
// is a prefix of p. When the file lives in multiple directories, the
// longest match wins so blog/2026/foo.md resolves to blog/ rather than blog/.
func (ve *HtmlViewEngine) matchedContentDir(p string) (string, bool) {
	var best string
	for _, d := range ve.contentDirs {
		if d == "" {
			continue
		}
		if p == d || strings.HasPrefix(p, d+"/") {
			if len(d) > len(best) {
				best = d
			}
		}
	}
	return best, best != ""
}

// loadContentFile reads a .md file, derives its ContentView (title from # H1,
// description from blockquote/paragraph), renders the body to HTML, stores
// the result in app.contentViews, locates an HTML template via bubble-up,
// and registers a route keyed by the .md slug (not the template path).
func (ve *HtmlViewEngine) loadContentFile(mdPath, dir string) error {
	buf, err := fs.ReadFile(ve.fsys, mdPath)
	if err != nil {
		return err
	}
	fi, _ := fs.Stat(ve.fsys, mdPath)

	var cv ContentView
	if ve.metaExtractor != nil {
		cv = ve.metaExtractor(mdPath, buf, fi)
	} else {
		cv = extractContentView(mdPath, buf, fi, dir, ve.md)
	}

	rendered, err := ve.renderMarkdown(buf, mdPath)
	if err != nil {
		ve.app.logger.Error("xun: render markdown", slog.String("path", mdPath), slog.Any("err", err))
		return err
	}
	cv.Body = rendered

	// Slug is path-relative-to-fsys-root, with the directory acting as
	// URL prefix. blog/post.md → /blog/post; docs/api/intro.md → /docs/api/intro.
	pattern := "GET " + path.Join("/", strings.TrimSuffix(mdPath, ".md"))
	ve.app.mu.Lock()
	ve.app.contentViews[pattern] = &cv
	ve.app.mu.Unlock()

	tmplPath := ve.bubbleUp(mdPath)
	if tmplPath == "" {
		ve.app.logger.Warn("xun: content has no bubble-up template",
			slog.String("path", mdPath), slog.String("slug", cv.Slug))
		return nil
	}

	// Load the bubble-up template if it hasn't been loaded yet. The key
	// preserves the extension so a sibling .tpl and .html at the same
	// path don't overwrite each other.
	tmplKey := ve.templateKey(tmplPath)
	t, ok := ve.templates[tmplKey]
	if !ok {
		var err error
		t, err = ve.loadContentTemplate(tmplPath, dir)
		if err != nil {
			return err
		}
	}

	// Register the route using the .md slug, not the template path.
	// Multiple .md files may share the same bubble-up template; each
	// gets its own route entry pointing at the shared HtmlTemplate.
	if _, exists := ve.app.routes[pattern]; !exists {
		ve.app.HandlePage(pattern, cv.Slug, &HtmlViewer{template: t})
	}
	return nil
}

// loadContentTemplate parses a template file (.tpl or .html) inside a
// content directory into ve.templates WITHOUT registering any route.
// It is used for:
//   - .tpl files: pure bubble-up targets that wrap sibling .md files.
//   - .html files: loaded here so loadContentPage can attach a route to
//     the same template; the route registration is performed separately.
//
// The template key is derived from the file path with its extension
// stripped so multiple .md files can share one bubble-up .tpl template.
func (ve *HtmlViewEngine) loadContentTemplate(tplPath, dir string) (*HtmlTemplate, error) {
	name := strings.TrimPrefix(tplPath, dir+"/")
	t := NewHtmlTemplate(name, tplPath)
	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
		return nil, err
	}
	ve.templates[ve.templateKey(tplPath)] = t
	return t, nil
}

// loadContentPage registers an HTML template living in a content directory
// as a page route in its own right, the same way loadPage does for pages/.
// This is used for .html files in content directories that are pages on
// their own (not bubble-up targets for .md files).
//
// In the current convention .html files in content/ are pages only — they
// are NEVER consulted by bubbleUp. That role belongs to .tpl files.
func (ve *HtmlViewEngine) loadContentPage(htmlPath, dir string) error {
	if htmlPath != dir && !strings.HasPrefix(htmlPath, dir+"/") {
		return nil
	}

	rel := strings.TrimPrefix(htmlPath, dir+"/")
	if rel == htmlPath {
		rel = ""
	}

	t, err := ve.loadContentTemplate(htmlPath, dir)
	if err != nil {
		return err
	}

	if strings.HasSuffix(htmlPath, "/index.html") {
		rel = strings.TrimSuffix(rel, "/index.html")
	}

	_, _, pattern := splitFile(rel)
	pattern = strings.TrimSuffix(pattern, ".html")

	ve.app.HandlePage(pattern, rel, &HtmlViewer{template: t})

	return nil
}

// bubbleUp finds the best matching bubble-up template for a .md file.
//
// Only .tpl files are candidates. .html files in content/ are page routes,
// not templates, so they are intentionally skipped from the lookup
// (so a directory such as content/blog/ can hold both an index.tpl template
// and an index.md page without the template occupying the route).
//
// Order (scoped to its own content directory):
//  1. <mdPath-without-.md>.tpl
//  2. <dir>/<sub>/index.tpl  (walking up the directory chain)
//  3. <dir>/index.tpl
//  4. root index.tpl
//
// Returns "" when no template is found.
func (ve *HtmlViewEngine) bubbleUp(mdPath string) string {
	full := strings.TrimSuffix(mdPath, ".md")

	if existsFS(ve.fsys, full+".tpl") {
		return full + ".tpl"
	}

	dir, _ := ve.matchedContentDir(mdPath)
	if dir == "" {
		return ""
	}

	cur := path.Dir(full)
	for cur != "." && cur != "/" && cur != "" && cur != dir {
		candidate := path.Join(cur, "index.tpl")
		if existsFS(ve.fsys, candidate) {
			return candidate
		}
		cur = path.Dir(cur)
	}

	if existsFS(ve.fsys, path.Join(dir, "index.tpl")) {
		return path.Join(dir, "index.tpl")
	}
	if existsFS(ve.fsys, "index.tpl") {
		return "index.tpl"
	}
	return ""
}

// templateKey returns the lookup key used to store *HtmlTemplate in
// ve.templates for files inside a content directory.
//
// Unlike loadTemplate (which strips .html from components/, layouts/,
// pages/, views/ keys), templateKey preserves the file extension so that
// a content/blog/index.tpl and a content/blog/index.html can coexist
// without overwriting each other. They are two separate templates with
// distinct roles — one is a bubble-up target, the other is a page.
//
// The legacy loadTemplate path keeps its stripped-name convention
// because components/layouts/pages/views never carry .tpl siblings.
func (ve *HtmlViewEngine) templateKey(p string) string {
	return p
}

// renderMarkdown dispatches to the user-provided renderFn when available,
// otherwise falls back to the default goldmark renderer.
func (ve *HtmlViewEngine) renderMarkdown(content []byte, path string) (template.HTML, error) {
	if ve.renderFn != nil {
		return ve.renderFn(content, path)
	}
	return ve.md.Render(content)
}

// existsFS returns true when the given path can be stat'd successfully.
func existsFS(fsys fs.FS, p string) bool {
	_, err := fs.Stat(fsys, p)
	return err == nil
}
