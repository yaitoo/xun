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

	// Content engine: loads .md files from contentDir, renders them to HTML,
	// and registers routes via bubble-up template lookup.
	md            *contentRenderer
	contentDir    string                                       // default "content"; empty disables
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

	if ve.contentDir == "" {
		ve.contentDir = "content"
	}
	if ve.md == nil {
		ve.md = newContentRenderer()
	}

	ve.loadComponents()
	ve.loadLayouts()
	ve.loadPages()
	ve.loadViews()
	ve.loadContentDir()
}

// FileChanged is called when a file has been changed.
//
// It is used to reload templates when they have been changed.
func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error { // skipcq: RVV-B0012

	if event.Has(fsnotify.Remove) {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(event.Name))

	switch ext {
	case ".html":
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
		case ve.contentDir != "" && strings.HasPrefix(event.Name, ve.contentDir+"/"):
			return ve.loadContentPage(event.Name)
		}
		return nil

	case ".md":
		if ve.contentDir == "" || !strings.HasPrefix(event.Name, ve.contentDir+"/") {
			return nil
		}
		if event.Has(fsnotify.Remove) {
			slug := strings.TrimSuffix(strings.TrimPrefix(event.Name, ve.contentDir+"/"), ".md")
			app.mu.Lock()
			delete(app.contentViews, "GET /"+slug)
			app.mu.Unlock()
			return nil
		}
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			return ve.loadContentFile(event.Name)
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

// loadContentDir walks the content directory and dispatches by file extension:
//   - .md   → loadContentFile (parse, render, register route via bubble-up)
//   - .html → loadContentPage (register as a regular page route)
//
// Subdirectories are walked recursively. Files outside the configured
// contentDir are ignored.
func (ve *HtmlViewEngine) loadContentDir() {
	if ve.contentDir == "" {
		return
	}
	fs.WalkDir(ve.fsys, ve.contentDir, func(p string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".md":
			if err := ve.loadContentFile(p); err != nil {
				ve.app.logger.Error("xun: load content", slog.String("path", p), slog.Any("err", err))
			}
		case ".html":
			if err := ve.loadContentPage(p); err != nil {
				ve.app.logger.Error("xun: load content page", slog.String("path", p), slog.Any("err", err))
			}
		}
		return nil
	})
}

// loadContentFile reads a .md file, derives its ContentView (title from # H1,
// description from blockquote/paragraph), renders the body to HTML, stores
// the result in app.contentViews, and ensures an HTML template exists via
// bubble-up lookup.
func (ve *HtmlViewEngine) loadContentFile(mdPath string) error {
	buf, err := fs.ReadFile(ve.fsys, mdPath)
	if err != nil {
		return err
	}
	fi, _ := fs.Stat(ve.fsys, mdPath)

	var cv ContentView
	if ve.metaExtractor != nil {
		cv = ve.metaExtractor(mdPath, buf, fi)
	} else {
		cv = extractContentView(mdPath, buf, fi, ve.contentDir, ve.md)
	}

	rendered, err := ve.renderMarkdown(buf, mdPath)
	if err != nil {
		ve.app.logger.Error("xun: render markdown", slog.String("path", mdPath), slog.Any("err", err))
		return err
	}
	cv.Body = rendered

	pattern := "GET /" + cv.Slug
	ve.app.mu.Lock()
	ve.app.contentViews[pattern] = &cv
	ve.app.mu.Unlock()

	tmplPath := ve.bubbleUp(cv.Slug)
	if tmplPath == "" {
		ve.app.logger.Warn("xun: content has no html template",
			slog.String("path", mdPath), slog.String("slug", cv.Slug))
		return nil
	}

	if _, ok := ve.templates[ve.templateKey(tmplPath)]; !ok {
		return ve.loadContentPage(tmplPath)
	}
	return nil
}

// loadContentPage registers an HTML template living in the content directory
// as a regular page route, the same way loadPage does for pages/.
func (ve *HtmlViewEngine) loadContentPage(htmlPath string) error {
	if !strings.HasPrefix(htmlPath, ve.contentDir+"/") && htmlPath != ve.contentDir {
		return nil
	}

	rel := strings.TrimPrefix(htmlPath, ve.contentDir+"/")
	if rel == htmlPath {
		rel = ""
	}

	name := rel
	t := NewHtmlTemplate(name, htmlPath)

	if err := t.Load(ve.fsys, ve.templates, ve.app.funcMap); err != nil {
		return err
	}

	ve.templates[ve.templateKey(htmlPath)] = t

	if strings.HasSuffix(htmlPath, "/index.html") {
		name = strings.TrimSuffix(name, "/index.html")
	}

	_, _, pattern := splitFile(name)
	pattern = strings.TrimSuffix(pattern, ".html")

	ve.app.HandlePage(pattern, htmlPath[len(ve.contentDir)+1:len(htmlPath)-5], &HtmlViewer{
		template: t,
	})

	return nil
}

// bubbleUp finds the best matching HTML template path for a slug.
//
// Order:
//  1. <contentDir>/<slug>.html
//  2. <contentDir>/<dir>/index.html  (walking up the directory chain)
//  3. content/index.html
//  4. index.html (root)
//
// Returns "" when no template is found.
func (ve *HtmlViewEngine) bubbleUp(slug string) string {
	full := path.Join(ve.contentDir, slug)

	if existsFS(ve.fsys, full+".html") {
		return full + ".html"
	}

	dir := path.Dir(full)
	for dir != "." && dir != "/" && dir != "" && dir != ve.contentDir {
		candidate := path.Join(dir, "index.html")
		if existsFS(ve.fsys, candidate) {
			return candidate
		}
		dir = path.Dir(dir)
	}

	if existsFS(ve.fsys, path.Join(ve.contentDir, "index.html")) {
		return path.Join(ve.contentDir, "index.html")
	}
	if existsFS(ve.fsys, "index.html") {
		return "index.html"
	}
	return ""
}

// templateKey returns the lookup key used to store *HtmlTemplate in ve.templates.
// For files in the content directory we use the full path so it does not collide
// with files in pages/, views/, layouts/, or components/.
func (ve *HtmlViewEngine) templateKey(p string) string {
	if strings.HasPrefix(p, ve.contentDir+"/") {
		return p[:len(p)-len(".html")]
	}
	return p[:len(p)-len(".html")]
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
