# Blog Engine Design

> Status: AUTHORITATIVE
> Before writing any xun blog code, read this entire document.

This document describes how xun supports Markdown-based blog posts on top of the existing `HtmlViewEngine`. The guiding principle is **reuse everything** — no new `Viewer` type, no new routing interface, no second template pipeline. Markdown is treated as another page whose HTML template lives next to it and whose content is injected via a new `ViewModel.Blog` field.

---

## Section 0 — Design Principles (Read Before Writing Any Code)

These principles are numbered. All subsequent sections reference them.

### Principle 0.1 — Reuse `HtmlViewEngine` infrastructure

The blog engine does not introduce a new `Viewer`, a new routing registration, or a parallel template pipeline. It extends `HtmlViewEngine` with three minimal additions:

1. A `*BlogView` field on `ViewModel` populated at render time.
2. A `map[string]*BlogView` on `App` keyed by route pattern.
3. A walker that handles `.md` files alongside `.html` files in the blog directory.

### Principle 0.2 — Derive metadata from Markdown semantics, not frontmatter

The blog engine does **not** parse YAML/TOML frontmatter. Title is derived from the first `# H1` heading. Description is derived from the first blockquote (preferred) or top-level paragraph. Everything else comes from the filesystem.

### Principle 0.3 — No blog-specific layout file

There is no `blog/_layout.html`, no `blog/_index.html`, no `blog/_tags.html`. Every HTML template in the blog directory is a standard `HtmlViewEngine` page that uses `<!--layout:NAME-->` to reference the same `layouts/` directory used by every other page on the site.

### Principle 0.4 — Markdown files are read at runtime

`.md` files are read from the user-provided `fs.FS` at startup and on file-change events. The engine never uses `//go:embed` for blog content.

---

## Section 1 — Directory Convention

```
layouts/                     ← Site-wide layouts (unchanged)
├── site.html                ← Referenced via <!--layout:site-->
└── ...

components/                  ← Site-wide components (unchanged)
├── header.html
└── footer.html

pages/                       ← Non-blog pages (unchanged)
├── about.html
└── contact.html

blog/                        ← Blog directory (default; configurable via WithBlogDir)
├── hello.md                 ← Auto-registered: GET /hello
├── hello.html               ← Optional specific template for /hello
├── 2026/
│   ├── index.html           ← Bubble-up target for sibling .md files
│   ├── deeper.md            ← Auto-registered: GET /2026/deeper
│   └── deeper.html          ← Optional specific template
└── about.md                 ← Auto-registered: GET /about
```

Three rules govern this layout:

**Rule 1.1 — Bubble-up template lookup**

For any route `R` derived from a `.md` file, the engine looks for an HTML template in this order:

1. `R` itself with `.html` extension (e.g. `blog/2026/deeper.html`)
2. The directory of `R` (e.g. `blog/2026/index.html`)
3. Each ancestor directory's `index.html` (e.g. `blog/index.html`)
4. The root `index.html`

**Rule 1.2 — Auto route registration for `.md`**

Every `.md` file in the blog directory auto-registers a route whose pattern is `GET /<slug>`. The `<slug>` is the file path relative to the blog directory, minus the `.md` extension.

**Rule 1.3 — Markdown content lives in `ViewModel.Blog`**

When a request matches a route that was registered from a `.md` file, the engine populates `ViewModel.Blog` with a `*BlogView`. Templates reference it as `{{.Blog.Title}}`, `{{.Blog.Content}}`, etc. Templates that do not match a blog route see `vm.Blog == nil`.

---

## Section 2 — Data Types

### 2.1 `BlogView` (defined in `markdown.go`)

```go
type BlogView struct {
    Path        string         // "blog/2026/deeper.md"
    Slug        string         // "2026/deeper"
    Title       string         // First # H1; empty if none
    Description string         // First blockquote (preferred) or top-level paragraph; empty if none
    Date        time.Time      // File mtime
    Content     template.HTML  // Rendered Markdown
}
```

Six fields. Two (`Title`, `Description`) come from Markdown semantics. Three (`Path`, `Slug`, `Date`) come from the filesystem. One (`Content`) comes from goldmark rendering.

### 2.2 `ViewModel` (defined in `viewer.go`, extended)

```go
type ViewModel struct {
    TempData map[string]any
    Data     any
    Blog     *BlogView  // NEW: nil when the route has no associated Markdown
}
```

The new field is optional. Existing templates that never reference `.Blog` continue to work unchanged.

### 2.3 No `BlogMeta` struct

Earlier drafts proposed a `BlogMeta` struct with fourteen YAML fields. That struct is removed. Anything beyond the six fields of `BlogView` is reachable via user-provided hooks (`WithBlogMeta`) or `WithTemplateFunc`.

---

## Section 3 — The Markdown Module (`markdown.go`)

The entire module is roughly 60 lines. It contains one type, one constructor, two methods, and one helper function.

### 3.1 Renderer

```go
type markdownRenderer struct {
    md     goldmark.Markdown
    parser parser.Parser
}

func newMarkdownRenderer() *markdownRenderer {
    md := goldmark.New(
        goldmark.WithExtensions(extension.GFM),
    )
    return &markdownRenderer{
        md:     md,
        parser: md.Parser(),
    }
}
```

The renderer enables only `extension.GFM` (GitHub Flavored Markdown). No AST transformers, no link rewriters, no typographer. Users who need more bring their own via `WithBlogRenderer`.

The `parser` field is exposed so `Extract` and `Render` share the same parse output.

### 3.2 Extract

```go
func (r *markdownRenderer) Extract(content []byte) (title, description string)
```

Walks the parsed AST once. Rules:

- **Title**: first `*ast.Heading` with `Level == 1`. Its accumulated text becomes `Title`. Code blocks, fenced code blocks, and raw HTML are skipped via `WalkSkipChildren` so `#` inside them cannot become a title.
- **Description**: after the title is found, the first `*ast.Blockquote` is preferred; otherwise the first `*ast.Paragraph` whose parent is the document root becomes `Description`. Nested paragraphs (inside lists or blockquotes) are ignored.
- Both return empty strings if not found. The caller decides what to do with empty values.

### 3.3 Render

```go
func (r *markdownRenderer) Render(content []byte) (template.HTML, error)
```

Calls `r.md.Convert(content, &buf)` and wraps the result as `template.HTML`. Because the goldmark output is already escaped by goldmark itself, `html/template` will not re-escape it.

### 3.4 extractBlogMeta

```go
func extractBlogMeta(mdPath string, content []byte, fi fs.FileInfo, r *markdownRenderer) BlogView
```

Combines filesystem info and Markdown semantics:

- `Path` = `mdPath` (verbatim)
- `Slug` = `mdPath` minus blog directory prefix and `.md` extension
- `Date` = `fi.ModTime()` if `fi` is non-nil, else zero
- `Title`, `Description` = `r.Extract(content)`
- `Content` = empty (filled by `loadBlogContent` after rendering)

This function has zero external dependencies beyond `fs.FileInfo` and the renderer.

---

## Section 4 — Integration with `HtmlViewEngine`

The blog engine is not a separate engine. It is a set of methods and one field added to `HtmlViewEngine`.

### 4.1 New field on `HtmlViewEngine`

```go
type HtmlViewEngine struct {
    // ... existing fields ...
    md            *markdownRenderer
    blogDir       string                                      // default "blog"
    metaExtractor func(string, []byte, fs.FileInfo) BlogView  // nil → use extractBlogMeta
    renderFn      func([]byte, string) (template.HTML, error) // nil → use md.Render
}
```

### 4.2 New methods

| Method | Purpose |
|--------|---------|
| `loadBlogDir()` | Walks `blog/` for `.md` and `.html` files |
| `loadBlogContent(mdPath)` | Loads one `.md` file into `app.blogPosts` |
| `loadBlogPage(htmlPath)` | Loads one `.html` file from `blog/` (analogous to `loadPage`) |
| `bubbleUp(slug)` | Pure function: returns the best matching template path |

### 4.3 Load flow

`HtmlViewEngine.Load` gains one line:

```go
func (ve *HtmlViewEngine) Load(fsys fs.FS, app *App) {
    // ... existing setup ...
    ve.md = newMarkdownRenderer()

    ve.loadComponents()
    ve.loadLayouts()
    ve.loadPages()
    ve.loadViews()
    ve.loadBlogDir() // NEW
}
```

`loadBlogDir` dispatches by extension:

```go
func (ve *HtmlViewEngine) loadBlogDir() {
    if ve.blogDir == "" { return }
    fs.WalkDir(ve.fsys, ve.blogDir, func(p string, d fs.DirEntry, _ error) error {
        if d == nil || d.IsDir() { return nil }
        ext := strings.ToLower(filepath.Ext(p))
        switch ext {
        case ".md":  return ve.loadBlogContent(p)
        case ".html": return ve.loadBlogPage(p)
        }
        return nil
    })
}
```

### 4.4 `loadBlogContent` — full lifecycle of one `.md`

```go
func (ve *HtmlViewEngine) loadBlogContent(mdPath string) error {
    // 1. Read file + stat
    buf, err := fs.ReadFile(ve.fsys, mdPath)
    if err != nil { return err }
    fi, _ := fs.Stat(ve.fsys, mdPath)

    // 2. Extract metadata (filesystem + Markdown AST)
    var bv BlogView
    if ve.metaExtractor != nil {
        bv = ve.metaExtractor(mdPath, buf, fi)
    } else {
        bv = extractBlogMeta(mdPath, buf, fi, ve.md)
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
    bv.Content = rendered

    // 4. Store in app.blogPosts, keyed by route pattern
    pattern := "GET /" + bv.Slug
    ve.app.mu.Lock()
    ve.app.blogPosts[pattern] = &bv
    ve.app.mu.Unlock()

    // 5. Bubble-up to find an HTML template
    tmplPath := ve.bubbleUp(bv.Slug)
    if tmplPath == "" {
        ve.app.logger.Warn("xun: blog post has no html template",
            slog.String("path", mdPath), slog.String("slug", bv.Slug))
        return nil
    }

    // 6. Ensure template loaded (loadBlogPage may have already handled it)
    if _, ok := ve.templates[ve.templateKey(tmplPath)]; !ok {
        return ve.loadBlogPage(tmplPath)
    }
    return nil
}
```

### 4.5 `bubbleUp` — template lookup

```go
// bubbleUp finds the best HTML template for a slug.
//
// For slug "2026/deeper", in order:
//   1. blog/2026/deeper.html
//   2. blog/2026/index.html
//   3. blog/index.html
//   4. index.html
func (ve *HtmlViewEngine) bubbleUp(slug string) string {
    full := path.Join(ve.blogDir, slug)

    if existsFS(ve.fsys, full+".html") {
        return full + ".html"
    }

    dir := path.Dir(full)
    for dir != "." && dir != "/" && dir != "" {
        candidate := path.Join(dir, "index.html")
        if existsFS(ve.fsys, candidate) {
            return candidate
        }
        dir = path.Dir(dir)
    }

    if existsFS(ve.fsys, "index.html") {
        return "index.html"
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
    mu         sync.RWMutex          // NEW: protects blogPosts
    blogPosts  map[string]*BlogView  // NEW: key is route pattern e.g. "GET /2026/deeper"
}

// lookupBlog returns the BlogView for a given route pattern, or nil.
func (app *App) lookupBlog(pattern string) *BlogView {
    app.mu.RLock()
    defer app.mu.RUnlock()
    return app.blogPosts[pattern]
}
```

The mutex is required because `FileChanged` may run from a goroutine while a request handler is reading `blogPosts`.

---

## Section 6 — `ViewModel` and `HtmlViewer` changes

### 6.1 ViewModel field

```go
type ViewModel struct {
    TempData map[string]any
    Data     any
    Blog     *BlogView // NEW
}
```

### 6.2 HtmlViewer.Render

The existing `Render` method is modified to populate `vm.Blog`:

```go
func (v *HtmlViewer) Render(ctx *Context, data any) error {
    ctx.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
    if ctx.Request.Method == http.MethodHead {
        return nil
    }

    buf := BufPool.Get()
    defer BufPool.Put(buf)

    vm := ViewModel{TempData: ctx.TempData, Data: data}
    if bv := ctx.App.lookupBlog(ctx.Routing.Pattern); bv != nil { // NEW
        vm.Blog = bv
    }

    if err := v.template.Execute(buf, vm); err != nil { // changed: was ViewModel{TempData, Data}
        return err
    }
    _, err := buf.WriteTo(ctx.Response)
    return err
}
```

Net diff: three lines added, one line changed. Backward compatible because `Blog` defaults to `nil` for non-blog routes.

---

## Section 7 — Hot Reload

`HtmlViewEngine.FileChanged` is extended with a `.md` branch:

```go
func (ve *HtmlViewEngine) FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error {
    // ... existing .html handling ...

    if !strings.EqualFold(filepath.Ext(event.Name), ".md") { return nil }
    if !strings.HasPrefix(event.Name, ve.blogDir+"/") { return nil }

    if event.Has(fsnotify.Remove) {
        slug := strings.TrimSuffix(strings.TrimPrefix(event.Name, ve.blogDir+"/"), ".md")
        app.mu.Lock()
        delete(app.blogPosts, "GET /"+slug)
        app.mu.Unlock()
        return nil
    }

    if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
        return ve.loadBlogContent(event.Name) // idempotent
    }
    return nil
}
```

`loadBlogContent` is idempotent: calling it twice on the same path overwrites `app.blogPosts[pattern]` with a fresh `*BlogView`. The `*HtmlTemplate` for the page is not rebuilt (its source has not changed), so no route re-registration is needed.

Layout changes are handled by the existing `HtmlTemplate.Reload` machinery — the reverse-dependency graph automatically cascades reloads to every page that references the changed layout.

---

## Section 8 — Complete Data Flow

```
PHASE A — App Startup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
xun.New(opts...)
  └─ for ve in app.engines: ve.Load(app.fsys, app)
       └─ HtmlViewEngine.Load
            ├─ loadComponents()  → ve.templates["components/*"]
            ├─ loadLayouts()          → ve.templates["layouts/*"]
            ├─ loadPages()            → ve.templates["pages/*"] + app.routes
            ├─ loadViews()            → app.viewers["views/*"]
            └─ loadBlogDir()          → split by extension
                 ├─ .html → loadBlogPage  → ve.templates["blog/*"] + app.routes
                 └─ .md   → loadBlogContent
                            ├─ fs.ReadFile + fs.Stat
                            ├─ md.Extract (AST) → Title, Description
                            ├─ md.Render (AST)  → Content
                            ├─ app.blogPosts["GET /<slug>"] = *BlogView
                            └─ bubbleUp → loadBlogPage if not yet loaded

PHASE B — Request Handling
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
GET /2026/deeper
  └─ http.ServeMux matches registered pattern
       └─ muxHandler closure
            ├─ rw := app.createWriter(req, w)   (gzip/deflate wrapper)
            ├─ ctx := &Context{ Routing: *r, App: app, TempData: {} }
            └─ r.Next(ctx) = middleware chain → r.Handle(ctx)
                 └─ HtmlViewer.Render(ctx, nil)
                      ├─ vm := ViewModel{ TempData, Data, Blog }
                      │   (Blog from app.lookupBlog(ctx.Routing.Pattern))
                      ├─ v.template.ExecuteTemplate(buf, layoutName, vm)
                      │   ├─ layouts/site.html (root layout)
                      │   │   ├─ {{template "components/header" .}}
                      │   │   ├─ {{template "content" .}}
                      │   │   │   └─ blog/2026/deeper.html
                      │   │   │       ├─ {{.Blog.Title}}
                      │   │   │       ├─ {{.Blog.Description}}
                      │   │   │       └─ {{.Blog.Content}} (goldmark output)
                      │   │   └─ {{template "components/footer" .}}
                      └─ buf.WriteTo(ctx.Response)
                           └─ HTTP/1.1 200 OK + full HTML
```

---

## Section 9 — Template Authoring

### 9.1 Blog page template

`blog/hello.html`:

```html
<!--layout:site-->

{{define "content"}}
<article class="blog-post">
  <header>
    <h1>{{.Blog.Title}}</h1>
    {{if .Blog.Description}}
      <p class="lede">{{.Blog.Description}}</p>
    {{end}}
    <time datetime="{{.Blog.Date.Format "2006-01-02"}}">
      {{.Blog.Date.Format "January 2, 2006"}}
    </time>
  </header>
  <div class="blog-content">{{.Blog.Content}}</div>
</article>
{{end}}
```

### 9.2 Markdown source

`blog/hello.md`:

```markdown
# Hello World

> A short summary of what this post is about.

This is the **content** with [a link](https://example.com).

## Section

More content here.
```

→ `BlogView`:

```
Path:        "blog/hello.md"
Slug:        "hello"
Title:       "Hello World"            ← from "# Hello World"
Description: "A short summary..."     ← from the blockquote
Date:        <file mtime>
Content:     "<p>This is the <strong>content</strong> with <a href=\"https://example.com\">a link</a>.</p>\n<h2>Section</h2>\n<p>More content here.</p>\n"
```

### 9.3 Markdown without `# H1`

If the source has no H1:

```markdown
This is content without a heading.

Some paragraph.
```

→ `BlogView.Title == ""`. Templates must guard with `{{if .Blog.Title}}` or use `{{.Blog.Title | default .Blog.Slug}}` patterns.

### 9.4 Plain (non-Markdown) page

A normal `pages/about.html` template is unchanged. `vm.Blog` is `nil`. Any reference to `.Blog.X` is a template-time error unless guarded.

---

## Section 10 — Edge Cases

| Scenario | Behavior |
|----------|----------|
| `blog/foo.md` with no `.html` fallback anywhere | Warning log. Route is not registered. Visiting `/foo` returns 404. |
| `blog/foo.md` + `blog/foo.html` | Route `/foo` uses `foo.html` as template. |
| `blog/foo.md` + `blog/index.html` | Route `/foo` uses `index.html` via bubble-up. |
| `blog/foo.md` + root `index.html` | Route `/foo` uses root `index.html` via bubble-up. |
| Markdown has no `# H1` | `BlogView.Title == ""`. Template must guard. |
| Markdown has no blockquote or paragraph | `BlogView.Description == ""`. Template must guard. |
| Markdown render fails (parser error) | Error log. `app.blogPosts` not written. Route not registered. |
| Multiple `.md` with same slug | Last one wins (map write semantics). |
| `HEAD` request | `Render` returns early after setting `Content-Type`. |
| `vm.Blog == nil` (non-blog route) | Any `{{.Blog.X}}` access is a template-time error. Use `{{if .Blog}}` to guard. |
| Concurrent `FileChanged` and request | `sync.RWMutex` on `app.blogPosts` protects the map. Read lock for `lookupBlog`, write lock for inserts/deletes. |

---

## Section 11 — Escape Hatches (Three Options)

The default implementation covers 90% of cases. Power users have three hooks, each replacing a default stage entirely:

```go
// 1. Replace the blog directory (default "blog")
func WithBlogDir(dir string) Option

// 2. Replace Markdown rendering entirely
func WithBlogRenderer(
    render func(content []byte, path string) (template.HTML, error),
) Option

// 3. Replace metadata extraction entirely
func WithBlogMeta(
    fn func(path string, content []byte, fi fs.FileInfo) BlogView,
) Option
```

These are the **only** user-facing options. Everything else is fixed by the principles above.

---

## Section 12 — Implementation Roadmap

The implementation is split into seven independent steps, each committable and verifiable:

| Step | Files | Verification |
|------|-------|--------------|
| 1. Write `markdown.go` | new file | `go test ./...` passes (no regressions) |
| 2. Add `Blog *BlogView` to `ViewModel` | `viewer.go` | Existing template tests pass |
| 3. Add blog injection to `HtmlViewer.Render` | `viewer_html.go` | Existing tests pass |
| 4. Add `blogPosts` + `lookupBlog` to `App` | `app.go` | Existing tests pass |
| 5. Add `loadBlogDir` / `loadBlogContent` / `loadBlogPage` / `bubbleUp` to `HtmlViewEngine` | `viewengine_html.go` | Existing tests pass; new tests for `bubbleUp` pass |
| 6. Extend `FileChanged` for `.md` events | `viewengine_html.go` | Hot-reload manual test |
| 7. Add the three Options | `option.go` | Compile check |

After step 7, write `markdown_test.go` covering Extract edge cases (H1 in code blocks, nested paragraphs, no H1, blockquote preferred) and Render smoke tests.

---

## Section 13 — Summary

The blog engine is a **seven-file, ~150-line change** to `HtmlViewEngine`. It adds:

- One new file (`markdown.go`)
- Three new fields (`ViewModel.Blog`, `App.blogPosts`, `HtmlViewEngine.md`)
- Four new methods (`loadBlogDir`, `loadBlogContent`, `loadBlogPage`, `bubbleUp`)
- Three new options (`WithBlogDir`, `WithBlogRenderer`, `WithBlogMeta`)
- One hot-reload extension (`.md` branch in `FileChanged`)

It does not add any new `Viewer`, any new routing interface, any new template pipeline, or any second layout directory. The user's mental model is: "Markdown files are pages whose content lives in `.Blog.*`."