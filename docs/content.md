# Content Engine Design

> Status: AUTHORITATIVE
> Before writing any xun content code, read this entire document.

This document describes how xun serves Markdown files as dynamic pages on top of the existing `HtmlViewEngine`. The mechanism is **deliberately generic**: it works for blog posts, documentation, knowledge-base articles, changelogs, or any other use case where content lives in Markdown files rather than a database. The guiding principle is **reuse everything** — no new `Viewer` type, no new routing interface, no second template pipeline. Markdown is treated as another page whose HTML template lives next to it and whose content is injected via a new `ViewModel.Content` field.

---

## Section 0 — Design Principles (Read Before Writing Any Code)

These principles are numbered. All subsequent sections reference them.

### Principle 0.1 — Reuse `HtmlViewEngine` infrastructure

The content engine does not introduce a new `Viewer`, a new routing registration, or a parallel template pipeline. It extends `HtmlViewEngine` with three minimal additions:

1. A `*ContentView` field on `ViewModel` populated at render time.
2. A `map[string]*ContentView` on `App` keyed by route pattern.
3. A walker that handles `.md` files alongside `.html` files in the content directory.

### Principle 0.2 — Derive metadata from Markdown semantics, not frontmatter

The content engine does **not** parse YAML/TOML frontmatter. Title is derived from the first `# H1` heading. Description is derived from the first blockquote (preferred) or top-level paragraph. Everything else comes from the filesystem.

### Principle 0.3 — Templates and pages use distinct extensions

Inside `content/`, `.tpl` files are **bubble-up templates only** — they are loaded into the template graph but never registered as routes. `.html` files are **pages only** — they register a route and are not consulted during bubble-up lookup. This separation is what lets a directory such as `content/blog/` carry both a wrapping template (`index.tpl`) and a real Markdown page (`index.md`) without the template occupying the route.

### Principle 0.4 — Markdown files are read at runtime

`.md` files are read from the user-provided `fs.FS` at startup and on file-change events. The engine never uses `//go:embed` for content.

### Principle 0.5 — The engine does not inject HTML structure

The engine emits clean, class-less HTML from goldmark. **No wrapper, no auto-classes, no inline styles**. Templates are responsible for wrapping content in whatever scope the site's CSS requires. See Section 9.5 for details and rationale.

---

## Section 1 — Directory Convention

```
layouts/                     ← Site-wide layouts (unchanged)
├── site.html                ← Referenced via <!--layout:site-->
└── ...

components/                  ← Site-wide components (unchanged)
├── header.html
└── footer.html

pages/                       ← Non-content pages (unchanged)
├── about.html
└── contact.html

content/                     ← Content directory (default; configurable via WithContentDir)
├── hello.md                 ← Auto-registered: GET /hello
├── hello.tpl                ← Bubble-up template for /hello (optional, sibling .md)
├── hello.html               ← Standalone page for /hello (optional, sibling .md)
├── 2026/
│   ├── index.tpl            ← Bubble-up template for sibling .md files (no route)
│   ├── index.md             ← Directory-level page: GET /2026/  (own H1, breadcrumb anchor)
│   ├── deeper.md            ← Auto-registered: GET /2026/deeper
│   └── deeper.tpl           ← Optional specific template for /2026/deeper
└── about.md                 ← Auto-registered: GET /about
```

Three rules govern this layout:

**Rule 1.1 — Bubble-up template lookup (`.tpl` only)**

For any route `R` derived from a `.md` file, the engine looks for a bubble-up template in this order:

1. `R` itself with `.tpl` extension (e.g. `content/2026/deeper.tpl`)
2. The directory of `R` (e.g. `content/2026/index.tpl`)
3. Each ancestor directory's `index.tpl` (e.g. `content/index.tpl`)
4. The root `index.tpl`

`.html` files in `content/` are not consulted during bubble-up. The directory-level page at `/2026/` exists **independently** of the bubble-up template at `content/2026/index.tpl`.

**Rule 1.2 — Auto route registration for `.md`**

Every `.md` file in the content directory auto-registers a route whose pattern is `GET /<slug>`. The `<slug>` is the file path relative to the content directory, minus the `.md` extension.

**Rule 1.3 — Content lives in `ViewModel.Content`**

When a request matches a route that was registered from a `.md` file, the engine populates `ViewModel.Content` with a `*ContentView`. Templates reference it as `{{.Content.Title}}`, `{{.Content.Body}}`, etc. Templates that do not match a content route see `vm.Content == nil`.

---

## Section 2 — Data Types

### 2.1 `ContentView` (defined in `content.go`)

```go
type ContentView struct {
    Path        string         // "content/2026/deeper.md"
    Slug        string         // "2026/deeper"
    Title       string         // First # H1; empty if none
    Description string         // First blockquote (preferred) or top-level paragraph; empty if none
    Date        time.Time      // File mtime
    Body        template.HTML  // Rendered Markdown
}
```

Six fields. Two (`Title`, `Description`) come from Markdown semantics. Three (`Path`, `Slug`, `Date`) come from the filesystem. One (`Body`) comes from goldmark rendering.

### 2.2 `ViewModel` (defined in `viewer.go`, extended)

```go
type ViewModel struct {
    TempData map[string]any
    Data     any
    Content  *ContentView  // NEW: nil when the route has no associated Markdown
}
```

The new field is optional. Existing templates that never reference `.Content` continue to work unchanged.

### 2.3 No `ContentMeta` struct

Earlier drafts proposed a `ContentMeta` struct with many YAML fields. That struct is removed. Anything beyond the six fields of `ContentView` is reachable via user-provided hooks (`WithContentMeta`) or `WithTemplateFunc`.

---

## Section 3 — The Content Module (`content.go`)

The entire module is roughly 60 lines. It contains one type, one constructor, two methods, and one helper function.

### 3.1 Renderer

```go
type contentRenderer struct {
    md     goldmark.Markdown
    parser parser.Parser
}

func newContentRenderer() *contentRenderer {
    md := goldmark.New(
        goldmark.WithExtensions(extension.GFM),
    )
    return &contentRenderer{
        md:     md,
        parser: md.Parser(),
    }
}
```

The renderer enables only `extension.GFM` (GitHub Flavored Markdown). No AST transformers, no link rewriters, no typographer. Users who need more bring their own via `WithContentRenderer`.

The `parser` field is exposed so `Extract` and `Render` share the same parse output.

### 3.2 Extract

```go
func (r *contentRenderer) Extract(content []byte) (title, description string)
```

Walks the parsed AST once. Rules:

- **Title**: first `*ast.Heading` with `Level == 1`. Its accumulated text becomes `Title`. Code blocks, fenced code blocks, and raw HTML are skipped via `WalkSkipChildren` so `#` inside them cannot become a title.
- **Description**: after the title is found, the first `*ast.Blockquote` is preferred; otherwise the first `*ast.Paragraph` whose parent is the document root becomes `Description`. Nested paragraphs (inside lists or blockquotes) are ignored.
- Both return empty strings if not found. The caller decides what to do with empty values.

### 3.3 Render

```go
func (r *contentRenderer) Render(content []byte) (template.HTML, error)
```

Calls `r.md.Convert(content, &buf)` and wraps the result as `template.HTML`. Because the goldmark output is already escaped by goldmark itself, `html/template` will not re-escape it.

### 3.4 extractContentView

```go
func extractContentView(mdPath string, content []byte, fi fs.FileInfo, r *contentRenderer) ContentView
```

Combines filesystem info and Markdown semantics:

- `Path` = `mdPath` (verbatim)
- `Slug` = `mdPath` minus content directory prefix and `.md` extension
- `Date` = `fi.ModTime()` if `fi` is non-nil, else zero
- `Title`, `Description` = `r.Extract(content)`
- `Body` = empty (filled by `loadContentFile` after rendering)

This function has zero external dependencies beyond `fs.FileInfo` and the renderer.

---

## Section 4 — Integration with `HtmlViewEngine`

The content engine is not a separate engine. It is a set of methods and one field added to `HtmlViewEngine`.

### 4.1 New field on `HtmlViewEngine`

```go
type HtmlViewEngine struct {
    // ... existing fields ...
    md            *contentRenderer
    contentDir    string                                      // default "content"
    metaExtractor func(string, []byte, fs.FileInfo) ContentView // nil → use extractContentView
    renderFn      func([]byte, string) (template.HTML, error)  // nil → use md.Render
}
```

### 4.2 New methods

| Method | Purpose |
|--------|---------|
| `loadContentDir()` | Walks `content/` for `.md` and `.html` files |
| `loadContentFile(mdPath)` | Loads one `.md` file into `app.contentViews` |
| `loadContentPage(htmlPath)` | Loads one `.html` file from `content/` (analogous to `loadPage`) |
| `bubbleUp(slug)` | Pure function: returns the best matching template path |

### 4.3 Load flow

`HtmlViewEngine.Load` gains one line:

```go
func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) {
    // ... existing setup ...
    ve.md = newContentRenderer()

    ve.loadComponents()
    ve.loadLayouts()
    ve.loadPages()
    ve.loadViews()
    ve.loadContentDir() // NEW
}
```

`loadContentDir` dispatches by extension:

```go
func (ve *HtmlViewEngine) loadContentDir() {
    if ve.contentDir == "" { return }
    fs.WalkDir(ve.fsys, ve.contentDir, func(p string, d fs.DirEntry, _ error) error {
        if d == nil || d.IsDir() { return nil }
        ext := strings.ToLower(filepath.Ext(p))
        switch ext {
        case ".md":  return ve.loadContentFile(p)
        case ".html": return ve.loadContentPage(p)
        }
        return nil
    })
}
```

### 4.4 `loadContentFile` — full lifecycle of one `.md`

```go
func (ve *HtmlViewEngine) loadContentFile(mdPath string) error {
    // 1. Read file + stat
    buf, err := fs.ReadFile(ve.fsys, mdPath)
    if err != nil { return err }
    fi, _ := fs.Stat(ve.fsys, mdPath)

    // 2. Extract metadata (filesystem + Markdown AST)
    var cv ContentView
    if ve.metaExtractor != nil {
        cv = ve.metaExtractor(mdPath, buf, fi)
    } else {
        cv = extractContentView(mdPath, buf, fi, ve.md)
    }

    // 3. Render Markdown → HTML
    var rendered template.HTML
    if ve.renderFn != nil {
        r, err := ve.renderFn(buf, mdPath)
        if err != nil { return err }
        rendered = r
    } else {
        r, err := ve.md.Render(buf)
        if err != nil {
            ve.app.logger.Error("xun: render markdown",
                slog.String("path", mdPath), slog.Any("err", err))
            return err
        }
        rendered = r
    }
    cv.Body = rendered

    // 4. Store in app.contentViews, keyed by route pattern
    pattern := "GET /" + cv.Slug
    ve.app.mu.Lock()
    ve.app.contentViews[pattern] = &cv
    ve.app.mu.Unlock()

    // 5. Bubble-up to find an HTML template
    tmplPath := ve.bubbleUp(cv.Slug)
    if tmplPath == "" {
        ve.app.logger.Warn("xun: content has no html template",
            slog.String("path", mdPath), slog.String("slug", cv.Slug))
        return nil
    }

    // 6. Ensure template loaded (loadContentPage may have already handled it)
    if _, ok := ve.templates[ve.templateKey(tmplPath)]; !ok {
        return ve.loadContentPage(tmplPath)
    }
    return nil
}
```

### 4.5 `bubbleUp` — template lookup

```go
// bubbleUp finds the best bubble-up template for a slug.
//
// For slug "2026/deeper", in order:
//   1. content/2026/deeper.tpl
//   2. content/2026/index.tpl
//   3. content/index.tpl
//   4. index.tpl
//
// .html files in the content tree are NOT candidates — they are page
// routes, not templates. Templates live exclusively under .tpl so a
// directory such as content/blog/ can hold both index.tpl (template)
// and index.md (real page at /blog/) without conflict.
func (ve *HtmlViewEngine) bubbleUp(slug string) string {
    full := path.Join(ve.contentDir, slug)

    if existsFS(ve.fsys, full+".tpl") {
        return full+".tpl"
    }

    dir := path.Dir(full)
    for dir != "." && dir != "/" && dir != "" {
        candidate := path.Join(dir, "index.tpl")
        if existsFS(ve.fsys, candidate) {
            return candidate
        }
        dir = path.Dir(dir)
    }

    if existsFS(ve.fsys, "index.tpl") {
        return "index.tpl"
    }
    return ""
}

func existsFS(fsys fs.FS, p string) bool {
    _, err := fs.Stat(fsys, p)
    return err == nil
}
```

---

## Section 5 — `App` changes

```go
type App struct {
    // ... existing fields ...
    mu           sync.RWMutex            // NEW: protects contentViews
    contentViews map[string]*ContentView // NEW: key is route pattern e.g. "GET /2026/deeper"
}

// lookupContent returns the ContentView for a given route pattern, or nil.
func (app *App) lookupContent(pattern string) *ContentView {
    app.mu.RLock()
    defer app.mu.RUnlock()
    return app.contentViews[pattern]
}
```

The mutex is required because `FileChanged` may run from a goroutine while a request handler is reading `contentViews`.

---

## Section 6 — `ViewModel` and `HtmlViewer` changes

### 6.1 ViewModel field

```go
type ViewModel struct {
    TempData map[string]any
    Data     any
    Content  *ContentView // NEW
}
```

### 6.2 HtmlViewer.Render

The existing `Render` method is modified to populate `vm.Content`:

```go
func (v *HtmlViewer) Render(ctx *Context, data any) error {
    ctx.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
    if ctx.Request.Method == http.MethodHead {
        return nil
    }

    buf := BufPool.Get()
    defer BufPool.Put(buf)

    vm := ViewModel{TempData: ctx.TempData, Data: data}
    if cv := ctx.App.lookupContent(ctx.Routing.Pattern); cv != nil { // NEW
        vm.Content = cv
    }

    if err := v.template.Execute(buf, vm); err != nil { // changed: was ViewModel{TempData, Data}
        return err
    }
    _, err := buf.WriteTo(ctx.Response)
    return err
}
```

Net diff: three lines added, one line changed. Backward compatible because `Content` defaults to `nil` for non-content routes.

---

## Section 7 — Hot Reload

`HtmlViewEngine.FileChanged` is extended with a `.md` branch:

```go
func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
    // ... existing .html handling ...

    if !strings.EqualFold(filepath.Ext(event.Name), ".md") { return nil }
    if !strings.HasPrefix(event.Name, ve.contentDir+"/") { return nil }

    if event.Has(fsnotify.Remove) {
        slug := strings.TrimSuffix(strings.TrimPrefix(event.Name, ve.contentDir+"/"), ".md")
        app.mu.Lock()
        delete(app.contentViews, "GET /"+slug)
        app.mu.Unlock()
        return nil
    }

    if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
        return ve.loadContentFile(event.Name) // idempotent
    }
    return nil
}
```

`loadContentFile` is idempotent: calling it twice on the same path overwrites `app.contentViews[pattern]` with a fresh `*ContentView`. The `*HtmlTemplate` for the page is not rebuilt (its source has not changed), so no route re-registration is needed.

Layout changes are handled by the existing `HtmlTemplate.Reload` machinery — the reverse-dependency graph automatically cascades reloads to every page that references the changed layout.

---

## Section 8 — Complete Data Flow

```
PHASE A — App Startup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
xun.New(opts...)
  └─ for ve in app.engines:ve.Load(app.fsys, app)
       └─ HtmlViewEngine.Load
            ├─ loadComponents()  → ve.templates["components/*"]
            ├─ loadLayouts()          → ve.templates["layouts/*"]
            ├─ loadPages()            → ve.templates["pages/*"] + app.routes
            ├─ loadViews()            → app.viewers["views/*"]
            └─ loadContentDir()       → split by extension
                 ├─ .html → loadContentPage → ve.templates["content/*"] + app.routes
                 └─ .md   → loadContentFile
                            ├─ fs.ReadFile + fs.Stat
                            ├─ md.Extract (AST) → Title, Description
                            ├─ md.Render (AST)  → Body
                            ├─ app.contentViews["GET /<slug>"] = *ContentView
                            └─ bubbleUp → loadContentPage if not yet loaded

PHASE B — Request Handling
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
GET /2026/deeper
  └─ http.ServeMux matches registered pattern
       └─ muxHandler closure
            ├─ rw := app.createWriter(req, w)   (gzip/deflate wrapper)
            ├─ ctx := &Context{ Routing: *r, App: app, TempData: {} }
            └─ r.Next(ctx) = middleware chain → r.Handle(ctx)
                 └─ HtmlViewer.Render(ctx, nil)
                      ├─ vm := ViewModel{ TempData, Data, Content }
                      │   (Content from app.lookupContent(ctx.Routing.Pattern))
                      ├─ v.template.ExecuteTemplate(buf, layoutName, vm)
                      │   ├─ layouts/site.html (root layout)
                      │   │   ├─ {{template "components/header" .}}
                      │   │   ├─ {{template "content" .}}
                      │   │   │   └─ content/2026/deeper.html
                      │   │   │       ├─ {{.Content.Title}}
                      │   │   │       ├─ {{.Content.Description}}
                      │   │   │       └─ {{.Content.Body}} (goldmark output)
                      │   │   └─ {{template "components/footer" .}}
                      └─ buf.WriteTo(ctx.Response)
                           └─ HTTP/1.1 200 OK + full HTML
```

---

## Section 9 — Template Authoring

### 9.1 Bubble-up template

`content/hello.tpl` (sibling to `content/hello.md`):

```html
<!--layout:site-->

{{define "content"}}
<article class="blog-post">
  <header>
    <h1>{{.Content.Title}}</h1>
    {{if .Content.Description}}
      <p class="lede">{{.Content.Description}}</p>
    {{end}}
    <time datetime="{{.Content.Date.Format "2006-01-02"}}">
      {{.Content.Date.Format "January 2, 2006"}}
    </time>
  </header>
  <div class="content-body">
    {{.Content.Body}}
  </div>
</article>
{{end}}
```

The `.tpl` extension tells the engine: load this as a template that wraps sibling `.md` content, **do not** register it as a route. `content/hello.html` would behave differently — it would be a standalone page at `GET /hello` and would never be picked up by `bubbleUp`.

Note the `<div class="content-body">` wrapper. See Section 9.5 for why and how to style it.

### 9.2 Markdown source

`content/hello.md`:

```markdown
# Hello World

> A short summary of what this post is about.

This is the **content** with [a link](https://example.com).

## Section

More content here.
```

→ `ContentView`:

```
Path:        "content/hello.md"
Slug:        "hello"
Title:       "Hello World"            ← from "# Hello World"
Description: "A short summary..."     ← from the blockquote
Date:        <file mtime>
Body:        "<p>This is the <strong>content</strong> with <a href=\"https://example.com\">a link</a>.</p>\n<h2>Section</h2>\n<p>More content here.</p>\n"
```

### 9.3 Markdown without `# H1`

If the source has no H1:

```markdown
This is content without a heading.

Some paragraph.
```

→ `ContentView.Title == ""`. Templates must guard with `{{if .Content.Title}}` or use `{{.Content.Title | default .Content.Slug}}` patterns.

### 9.4 Plain (non-Markdown) page

A normal `pages/about.html` template is unchanged. `vm.Content` is `nil`. Any reference to `.Content.X` is a template-time error unless guarded.

### 9.5 Styling Content

This section is **mandatory reading** for anyone who wants Markdown content to look like the rest of the site.

#### 9.5.1 What the engine produces

goldmark converts Markdown into **clean, class-less, semantic HTML**. For example:

```markdown
# Title

> Quote

**Bold** with [link](url).

- item1
- item 2

```go
fmt.Println("hi")
```
```

becomes:

```html
<h1>Title</h1>
<blockquote><p>Quote</p></blockquote>
<p><strong>Bold</strong> with <a href="url">link</a>.</p>
<ul><li>item 1</li><li>item 2</li></ul>
<pre><code class="language-go">fmt.Println("hi")
</code></pre>
```

There is no wrapper element, no auto-injected class, no inline style. The engine treats Markdown as content, not as a UI component.

#### 9.5.2 Why the engine stays out of styling

Every major Markdown-based static site system follows the same pattern:

| System | Markdown output | Style responsibility |
|--------|-----------------|----------------------|
| Hugo | clean HTML | theme wraps + provides CSS |
| GitBook | clean HTML | theme wraps in `<div class="markdown-section">` + theme CSS |
| Jekyll | clean HTML | layout wraps + user CSS |
| Astro | clean HTML | `<article class="prose">` + Tailwind Typography |

None of them inject CSS at the engine level. The reason is structural: an engine does not know whether the user prefers Tailwind, vanilla CSS, Sass, CSS-in-JS, or a design system. Forcing a wrapper or a class into the engine output would lock users into one choice.

#### 9.5.3 The recommended pattern

Templates wrap `{{.Content.Body}}` in a scope-bearing element, and site CSS targets that scope:

```html
<div class="content-body">
  {{.Content.Body}}
</div>
```

```css
.content-body h1 { /* … */ }
.content-body p  { /* … */ }
```

This pattern is universal. Section 9.5.4 ships a ready-to-use CSS template; Section 9.5.5 shows the Tailwind alternative.

#### 9.5.4 Default CSS (GitHub-like)

Copy this into your site stylesheet to get a clean, readable baseline that matches GitHub's Markdown rendering:

```css
/* content/body — GitHub-like defaults; override as needed */
.content-body {
  line-height: 1.7;
  color: #24292f;
  font-size: 16px;
}
.content-body > * + * { margin-top: 1.2em; }

.content-body h1,
.content-body h2,
.content-body h3,
.content-body h4,
.content-body h5,
.content-body h6 {
  font-weight: 600;
  line-height: 1.25;
  margin-top: 1.6em;
  margin-bottom: 0.6em;
}
.content-body h1 { font-size: 2em;    border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
.content-body h2 { font-size: 1.5em;  border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
.content-body h3 { font-size: 1.25em; }
.content-body h4 { font-size: 1em;    }
.content-body h5 { font-size: 0.875em; }
.content-body h6 { font-size: 0.85em;  color: #57606a; }

.content-body p { margin: 1em 0; }
.content-body a { color: #0969da; text-decoration: none; }
.content-body a:hover { text-decoration: underline; }

.content-body ul,
.content-body ol { padding-left: 2em; margin: 1em 0; }
.content-body li + li { margin-top: 0.3em; }
.content-body li > p { margin: 0.4em 0; }

.content-body blockquote {
  border-left: 4px solid #d0d7de;
  padding: 0 1em;
  color: #57606a;
  margin: 1em 0;
}
.content-body blockquote > :first-child { margin-top: 0; }
.content-body blockquote > :last-child  { margin-bottom: 0; }

.content-body code {
  font-family: 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace;
  font-size: 0.9em;
  background: rgba(175, 184, 193, 0.2);
  padding: 0.2em 0.4em;
  border-radius: 6px;
}
.content-body pre {
  background: #f6f8fa;
  padding: 1em;
  border-radius: 6px;
  overflow-x: auto;
  margin: 1em 0;
}
.content-body pre code {
  background: transparent;
  padding: 0;
  font-size: 0.9em;
  border-radius: 0;
}

.content-body img { max-width: 100%; height: auto; }

.content-body table {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
  display: block;
  overflow-x: auto;
}
.content-body th,
.content-body td {
  border: 1px solid #d0d7de;
  padding: 6px 13px;
}
.content-body th {
  background: #f6f8fa;
  font-weight: 600;
}
.content-body tr:nth-child(even) td {
  background: #f6f8fa;
}

.content-body hr {
  border: 0;
  border-top: 1px solid #d0d7de;
  margin: 2em 0;
}
```

#### 9.5.5 Tailwind Typography (`.prose`)

If the site already uses Tailwind, install `@tailwindcss/typography` and wrap with `prose`:

```bash
npm install -D @tailwindcss/typography
```

```js
// tailwind.config.js
module.exports = {
  plugins: [require('@tailwindcss/typography')],
}
```

```html
<article class="prose prose-lg max-w-none dark:prose-invert">
  {{.Content.Body}}
</article>
```

No additional CSS needed. `prose` ships with GitHub-like defaults and is fully customizable via Tailwind config.

#### 9.5.6 Advanced: AST-level class injection

For per-element class injection (e.g., every `<h1>` gets `class="md-h1"`), provide a custom renderer via `WithContentRenderer`:

```go
xun.WithContentRenderer(func(content []byte, path string) (template.HTML, error) {
    md := goldmark.New(
        goldmark.WithExtensions(extension.GFM),
        goldmark.WithRendererOptions(
            html.WithAttributeFilter(), // honor {#id .class} syntax in Markdown
        ),
    )
    // Optional: register a custom NodeRendererFuncSet that adds classes per element
    // See https://github.com/yuin/goldmark/blob/master/renderer/html for details.

    var buf bytes.Buffer
    if err := md.Convert(content, &buf); err != nil {
        return "", err
    }
    return template.HTML(buf.String()), nil
})
```

This is an escape hatch — most users never need it.

---

## Section 10 — Edge Cases

| Scenario | Behavior |
|----------|----------|
| `content/foo.md` with no `.tpl` fallback anywhere | Warning log. Route is not registered. Visiting `/foo` returns 404. |
| `content/foo.md` + `content/foo.tpl` | Route `/foo` uses `foo.tpl` as template. |
| `content/foo.md` + `content/index.tpl` | Route `/foo` uses `index.tpl` via bubble-up. |
| `content/foo.md` + root `index.tpl` | Route `/foo` uses root `index.tpl` via bubble-up. |
| `content/blog/index.tpl` + `content/blog/index.md` | Bubble-up template and a real directory-level page coexist. `GET /blog` renders `index.md` wrapped by `index.tpl`. |
| `content/blog/index.tpl` only (no `index.md`) | No route at `/blog`; only sibling `.md` files under `blog/` are reachable. |
| Markdown has no `# H1` | `ContentView.Title == ""`. Template must guard. |
| Markdown has no blockquote or paragraph | `ContentView.Description == ""`. Template must guard. |
| Markdown render fails (parser error) | Error log. `app.contentViews` not written. Route not registered. |
| Multiple `.md` with same slug | Last one wins (map write semantics). |
| `HEAD` request | `Render` returns early after setting `Content-Type`. |
| `vm.Content == nil` (non-content route) | Any `{{.Content.X}}` access is a template-time error. Use `{{if .Content}}` to guard. |
| Concurrent `FileChanged` and request | `sync.RWMutex` on `app.contentViews` protects the map. Read lock for `lookupContent`, write lock for inserts/deletes. |

---

## Section 11 — Escape Hatches (Three Options)

The default implementation covers 90% of cases. Power users have three hooks, each replacing a default stage entirely:

```go
// 1. Replace the content directory (default "content")
func WithContentDir(dir string) Option

// 2. Replace Markdown rendering entirely
func WithContentRenderer(
    render func(content []byte, path string) (template.HTML, error),
) Option

// 3. Replace metadata extraction entirely
func WithContentMeta(
    fn func(path string, content []byte, fi fs.FileInfo) ContentView,
) Option
```

These are the **only** user-facing options. Everything else is fixed by the principles above.

---

## Section 12 — Implementation Roadmap

The implementation is split into seven independent steps, each committable and verifiable:

| Step | Files | Verification |
|------|-------|--------------|
| 1. Write `content.go` | new file | `go test ./...` passes (no regressions) |
| 2. Add `Content *ContentView` to `ViewModel` | `viewer.go` | Existing template tests pass |
| 3. Add content injection to `HtmlViewer.Render` | `viewer_html.go` | Existing tests pass |
| 4. Add `contentViews` + `lookupContent` to `App` | `app.go` | Existing tests pass |
| 5. Add `loadContentDir` / `loadContentFile` / `loadContentPage` / `bubbleUp` to `HtmlViewEngine` | `viewengine_html.go` | Existing tests pass; new tests for `bubbleUp` pass |
| 6. Extend `FileChanged` for `.md` events | `viewengine_html.go` | Hot-reload manual test |
| 7. Add the three Options | `option.go` | Compile check |

After step 7, write `content_test.go` covering Extract edge cases (H1 in code blocks, nested paragraphs, no H1, blockquote preferred) and Render smoke tests.

---

## Section 13 — Summary

The content engine is a **seven-file, ~150-line change** to `HtmlViewEngine`. It adds:

- One new file (`content.go`)
- Three new fields (`ViewModel.Content`, `App.contentViews`, `HtmlViewEngine.md`)
- Four new methods (`loadContentDir`, `loadContentFile`, `loadContentPage`, `bubbleUp`)
- Three new options (`WithContentDir`, `WithContentRenderer`, `WithContentMeta`)
- One hot-reload extension (`.md` branch in `FileChanged`)

It does not add any new `Viewer`, any new routing interface, any new template pipeline, or any second layout directory. It does not inject any HTML structure into the Markdown output. The user's mental model is: "Markdown files are pages whose content lives in `.Content.*`."