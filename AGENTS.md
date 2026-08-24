# Xun Development Guide for AI Agents

A concise, code-accurate guide to use, extend, and safely modify xun without reading all source files.

## 1) Core concepts
- App: main instance holding ServeMux, routes, global middlewares, view engines, viewers, compressors, template func map, and asset URL map.
- Router/Group: register routes and middlewares. Uses Go 1.22 ServeMux patterns: "METHOD pattern".
- Middleware: func(next HandleFunc) HandleFunc.
- Context: wraps Request/Response, Routing, App, TempData. Provides View, Redirect, Accept, AcceptLanguage, Get/Set.
- Viewer: renders response by content negotiation (HTML/JSON/Text/XML/File/String).
- ViewEngine: loads templates/static files from fs.FS; supports hot-reload in dev.
- ResponseWriter: wraps http.ResponseWriter; tracks status/body bytes; supports gzip/deflate transparently.

## 2) App creation and lifecycle
Canonical six-step `main()` shape: load config → open resources → build app → register routes → start app → run listeners.

```go
err := loadConfig()
db, err := setupDB(ctx)

mux := http.NewServeMux()
app := xun.New(
    xun.WithMux(mux),                              // always pass your own mux; default pollutes http.DefaultServeMux
    xun.WithFsys(getFsys()),                       // dev: os.DirFS("./app"); prod: //go:embed sub
    xun.WithWatch(),                               // DEV ONLY — thread-unsafe
    xun.WithHandlerViewers(&xun.JsonViewer{}),     // content negotiation
    xun.WithInterceptor(htmx.New()),               // cross-cutting wrappers
    xun.WithBuildAssetURL(func(p string) bool {
        return strings.HasPrefix(p, "/assets/")
    }),
    xun.WithCompressor(&xun.GzipCompressor{}, &xun.DeflateCompressor{}),
)

setupRoutes(app)        // xun-managed (go through middleware pipeline)
app.Start()             // non-blocking — registers handlers on mux
defer app.Close()
http.ListenAndServe(":80", mux)
```

Notes:
- `WithWatch` is not thread-safe; enable only in development. In production embed assets and disable Watch.
- Always pass `WithMux(http.NewServeMux())` to `xun.New`. Skipping this lets `app.Mux()` fall back to `http.DefaultServeMux` and pollutes global state.

#### Dev / prod twin fs.FS
```go
//go:embed app
var fsys embed.FS
func getFsys() fs.FS {
    if fi, err := os.Stat("./app"); err == nil && fi.IsDir() {
        return os.DirFS("./app")       // dev: live files
    }
    sub, _ := fs.Sub(fsys, "app")      // prod: bundled binary
    return sub
}
```

## 3) Project structure and file routing
- public: static assets -> GET /...; public/index.html -> GET /{$}.
- components: reusable html fragments.
- layouts: page layouts; choose via <!--layout:name--> at top of page.
- pages: filesystem-based page routing; pages/foo/index.html -> GET /foo/{$}.
- views: internal views (not auto-routed), referenced by Context.View with viewer name.
- text: text templates (text/template); MIME/charset auto-detected from filename/content.
- Dynamic segments: {var} in file/dir names, e.g. pages/user/{id}.html -> GET /user/{id}.
- Multiple hosts: top-level folder like pages/@abc.com/index.html -> GET abc.com/{$}.

## 4) Routing and handlers
- Handler signature: func(c *xun.Context) error
```go
app.Get("/users/{id}", func(c *xun.Context) error {
  id := c.Request.PathValue("id")
  return c.View(User{Name: id})
})
```
- Groups and middlewares:
```go
admin := app.Group("/admin")
admin.Use(authMiddleware)
admin.Get("/{$}", handler)
```
- Error contract:
  - return nil: response done.
  - return xun.ErrCancelled: stop chain (you already handled response/redirect/status).
  - return other error: framework writes 500 + X-Log-Id; if xun.ErrViewNotFound then 404.

#### PageRoute with path parameters
A page file with a `{var}` segment auto-registers a PageRoute with the matching Go 1.22 pattern. To supply the model, override the same pattern with `app.Get` (or `g.Get` on a group):

```go
// pages/user/{id}.html    →  auto-registers GET /user/{id}
app.Get("/user/{id}", func(c *xun.Context) error {
    id, _ := strconv.ParseInt(c.Request.PathValue("id"), 10, 64)
    var u User
    db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = ?", id).
        Scan(&u.ID, &u.Name, &u.Email)
    return c.View(u)        // ← .Data = u
})
```

> **Path syntax:** only `{id}` (Go 1.22 brace form) works. `:id` colon-style is NOT understood by xun.

#### The `{$}` root pattern
`pages/index.html` auto-registers `GET /{$}` (xun appends `{$}` to patterns ending in `/` so they only match with the trailing slash). To supply data to the root page you must register against the same pattern:

```go
// Right — matches the auto-registered PageRoute
app.Get("/{$}", handleLanding)

// Wrong — different pattern, will not be called for /
// app.Get("/", handleLanding)
```

The same rule applies to `pages/<group>/index.html` inside `app.Group(...)`.

## 5) Middleware order
- App.Use applies globally; Group.Use applies to that group only.
- Construction is inside-out; execution is outer to inner; handler last.

## 6) Content negotiation and Viewers
- A route can have multiple viewers; chosen by Accept header. If none matches, fallback to the first.
- Default viewer for handlers is JSON; override with WithHandlerViewers or per-route WithViewer.
- Use named viewer explicitly: c.View(data, "views/name") or "text/sitemap.xml".
- Built-ins:
  - HtmlViewer: text/html
  - JsonViewer: application/json
  - TextViewer: text/* (based on file MIME)
  - XmlViewer: text/xml
  - StringViewer: text/plain
  - FileViewer: static files with ETag/Cache-Control support

#### When to omit the viewer name in `c.View`
| Call | When to use |
|---|---|
| `c.View(data)` | On a PageRoute (`pages/<X>.html` auto-registered the viewer). xun negotiates `HtmlViewer` vs `JsonViewer` from `Accept`. |
| `c.View(data, "index")` | On a manually-registered route (`app.Get` / `g.Get`) that needs to pick a specific page viewer. |
| `c.View(data, "views/users/list")` | For shared partials under `app/views/` that have no owning route. |

## 7) Templates and data model
- HTML uses html/template; text uses text/template (no HTML auto-escaping, ensure safety).
- Layout selection: put `<!--layout:home-->` at page top for layouts/home.html.
- Template model: Viewer receives ViewModel{TempData, Data}; use `.Data` and `.TempData` in templates.
- Template funcs: register via WithTemplateFunc/WithTemplateFuncMap. In production, built-in `asset` resolves fingerprinted asset URLs.

#### `.Data` vs `.TempData` — pick the right one
The template's `.` is bound to `ViewModel{TempData, Data}`. Two distinct maps; mixing them produces silent empty values.

| | `.Data` | `.TempData` |
|---|---|---|
| **Holds** | The current page's main business object (`User`, `[]User`, anything passed as `c.View`'s first arg) | Cross-request auxiliary values (current `Session`, page `title`, error messages, anything set via `c.Set(k, v)`) |
| **Set in Go** | `c.View(data, name)` first arg | `c.Set(k, v)` / `c.Get(k)` (both operate on `TempData`) |
| **Accessed in templates** | `{{.Data.X}}` (or `{{.X}}` after `{{range .Data}}`) | `{{.TempData.X}}` |
| **Lifetime** | Single `c.View(...)` call | Whole request — middleware → handler → view all share the same map |

Rules of thumb:
1. Page-level business data → `.Data`. SQL query result? That's `.Data.users` (`{{range .Data}} … {{end}}`).
2. Cross-page auxiliary state → `.TempData`. Session, page title, error message, request-scoped flags.
3. Don't duplicate. If `c.Set("session", s)` already ran in middleware, don't also pass `session` in `c.View`'s data.
4. Viewers AND layouts both see `TempData`. A layout's `{{.TempData.title}}` and a page's `{{.TempData.session.Name}}` work the same way because the whole `ViewModel` is passed as the template's `.`.

## 8) Static assets and fingerprinting
- StaticViewEngine registers files under public/ as routes.
- Fingerprint flow:
  - Register matchers with WithBuildAssetURL(func(string) bool) for assets to fingerprint.
  - Engine creates content-ETag-based URLs and routes with Cache-Control: public, max-age=31536000, immutable.
  - Use `{{ asset "/app.js" }}` in templates to get the fingerprinted URL.

## 9) Compression and ResponseWriter
- Configure compressors with WithCompressor; chosen by Accept-Encoding or wildcard.
- Handlers automatically wrap ResponseWriter and defer Close() to flush encoders.

## 10) Redirects and Interceptor
- c.Redirect(url[, code]) sets Location and 302 by default.
- Interceptor can override Redirect and RequestReferer (useful with htmx, etc.).

## 11) i18n + forms/validation (ext/form)
- Binding: form.BindQuery[T], form.BindForm[T], form.BindJson[T].
- Validation: it.Validate(c.AcceptLanguage()...); default messages are English; add locales via universal-translator.
- Typical failure: write 400 and return xun.ErrCancelled.

## 12) Coexist page routes and handler viewers
```go
app.Get("/{$}", func(c *xun.Context) error {
  return c.View(map[string]string{"Name":"xun"})
}, xun.WithViewer(&xun.HtmlViewer{/* render via views/... if you set it */}))
```

## 13) Hot Reload (dev only)
- Enabled when fs.FS provided and WithWatch set:
  - StaticViewEngine: Create/Write under public/.
  - HtmlViewEngine: *.html under components/layouts/pages/views.
  - TextViewEngine: Create/Write under text/.

## 14) Errors and status
- ErrCancelled: stop middleware chain, response considered handled.
- ErrViewNotFound: no matching viewer and no fallback; framework emits 404.
- Unhandled errors: framework writes 500 and X-Log-Id for diagnostics.

## 15) Extensions overview (ext/*)
- acl: host/IP/CIDR/country filtering and optional redirects; supports config hot-reload.
- autotls: automatic ACME certificates and renewal.
- cache: caching helpers (see ext/cache).
- cookie: base64 and signed cookies.
- csrf: CSRF protection with optional JsToken.
- form: binding + validation + i18n.
- hsts: HSTS and HTTP->HTTPS redirect (use only when HTTPS is available).
- htmx: integration helpers and interceptor.
- proxyproto: PROXY protocol v1/v2 support for ListenAndServe(TLS).
- reqlog: configurable access log middleware.
- sse: Server-Sent Events sessions, push, broadcast, and shutdown.

Usage patterns are app.Use(...) and/or app.Get(..., ext.HandleFunc(...)). See README and tests.

## 16) Performance and concurrency notes
- Do not enable WithWatch in production.
- Prefer BufPool for rendering in custom Viewer/Engine to minimize allocations.
- Compressors create per-request writers; always rely on framework’s defer Close().

## 17) Implementing Middleware/Viewer/ViewEngine
- Middleware: minimal logic; on refusal write status (e.g., 401/403/400) and return xun.ErrCancelled; avoid draining Body or reset as needed.
- Viewer: implement MimeType and Render; set Content-Type; for HEAD don’t write body; use BufPool.
- ViewEngine: implement Load and FileChanged; scan fs.FS; on Create/Write do localized reload. HtmlTemplate supports dependency graph reload.

## 18) Conventions and pitfalls
- pages/* auto-register GET only; other methods require explicit handlers.
- pages/foo/index.html -> GET /foo/{$} (matches only with trailing slash).
- pages/index.html -> GET /{$} (NOT GET /). To supply data you must register against the same pattern.
- Only `{id}` brace-style path segments are understood; `:id` colon-style is not.
- Context.View with named viewer must still match Accept; otherwise negotiation iterates route viewers, then first as fallback.
- text/* uses text/template (no HTML escaping) – sanitize output as needed.
- FileViewer supports ETag/If-None-Match; when using embed FS, ETag is auto-computed.
- content/*.md auto-registers as GET /<slug>; the slug is the path relative to the content dir minus `.md`. HTML template lookup is bubble-up: `<slug>.html` → `<dir>/index.html` → ancestor `index.html` → root `index.html`.
- Content title/description come from the Markdown AST (first `# H1` / first blockquote-or-top-paragraph) — there is no frontmatter. Don't read frontmatter from `.md` files.
- `app.Mux()` is the raw-mux escape hatch: routes registered there bypass middleware, viewers, error mapping, and access logs. Don't use it for ordinary handlers.
- `WithWatch` is not thread-safe; production builds must NOT enable it.

## 19) Minimal production-ready example
```go
//go:embed app
var fsys embed.FS
func main(){
  var dev bool
  flag.BoolVar(&dev, "dev", false, "dev")
  flag.Parse()
  var opts []xun.Option
  if dev { opts = []xun.Option{xun.WithFsys(os.DirFS("./app")), xun.WithWatch()} } 
  else { v, _ := fs.Sub(fsys, "app"); opts = []xun.Option{xun.WithFsys(v)} }
  app := xun.New(opts..., xun.WithCompressor(&xun.GzipCompressor{}))
  app.Get("/{$}", func(c *xun.Context) error { return c.View(map[string]string{"hello":"xun"}) })
  app.Start(); defer app.Close()
  http.ListenAndServe(":80", http.DefaultServeMux)
}
```

## 20) Safe-change checklist for Agents
- Keep changes minimal; prefer additive Options/RoutingOptions over changing defaults.
- Do not enable WithWatch in production; don’t alter mux unless using WithMux explicitly.
- Preserve existing MIME/route/hot-reload semantics when editing Viewer/Engine.
- On handler/middleware errors, set status and return ErrCancelled to avoid 500 fallback.
- Run existing tests before submitting; keep style consistent with README/AGENTS.md.

## 21) `app.Mux()` — the raw-mux escape hatch
`app.Mux()` returns the underlying `*http.ServeMux`. Use it ONLY when you need the raw `http.ResponseWriter`: WebSocket / SSE upgrades (`http.Hijacker`), third-party `http.Handler` integrations (Prometheus, pprof, OpenTelemetry), or low-precedence 404 fallbacks.

```go
app.Mux().HandleFunc("GET /api/ws", wsHandler(hub))
```

### Bypass semantics — read carefully
| Property | `app.Get / Post / HandleFunc` | `app.Mux().Handle / HandleFunc` |
|---|---|---|
| Runs `app.Use` middleware | ✓ | **✗** |
| Runs `group.Use` middleware | ✓ | **✗** |
| Goes through viewer / content negotiation | ✓ | **✗** |
| Logs `X-Log-Id` | ✓ | **✗** |
| Listed in `app.Start()` startup log | ✓ | **✗** |
| Error → status mapping by xun | ✓ | **✗** |
| Auth (session, CSRF, etc.) | automatic | **must be re-implemented on the raw request** |

### Footguns
- **Always pass `WithMux(http.NewServeMux())`** to `xun.New`. If you skip it, `app.Mux()` falls back to `http.DefaultServeMux` and you pollute global state.
- Patterns auto-registered by xun (e.g. `GET /{$}` from `pages/index.html`) **cannot** be re-registered through `app.Mux()` — `ServeMux` panics on duplicate patterns.
- Registering on `app.Mux()` from multiple goroutines concurrently panics. Do all registration before serving starts.
- If the native handler needs the same auth as the rest of the app, replicate the session/cookie check on the raw `*http.Request`. There's no compile-time check that native and xun-managed auth agree — keep them in sync.

### Motivating use case — WebSocket
xun wraps the writer to do error-to-status mapping and middleware plumbing, but the wrapper has no `Hijacker()`. WebSocket upgrades therefore MUST go through `app.Mux()` and call `w.(http.Hijacker).Hijack()` themselves, then write the `101 Switching Protocols` response by hand.

## 22) Cheat sheet
| Need | API |
|---|---|
| Build the app | `xun.New(opts...)` |
| Add global middleware | `app.Use(mw...)` |
| Group routes | `g := app.Group("/prefix"); g.Use(authMw)` |
| Register a route | `app.Get`, `app.Post`, `app.Put`, `app.Delete`, `app.HandleFunc` |
| Render page on a PageRoute | `c.View(data)` — viewer is on the route already |
| Render page on a manual route | `c.View(data, "X")` — explicit viewer key |
| Render a shared partial | `c.View(data, "views/X")` — must pass name (no owning route) |
| Pick a layout | `<!--layout:NAME-->` first line of the page |
| Layout → component | `{{block "components/<name>" .}}{{end}}` — must match file path |
| Component → component | `{{template "<name>" .}}` — base name, no `components/` prefix |
| Page body block | `{{define "content"}}...{{end}}` — required in every page |
| Optional page block | Override default with `{{define "name"}}`; otherwise layout default is used (Go std `html/template` behaviour) |
| Read form field | `c.Request.FormValue("k")` |
| Read path param | `c.Request.PathValue("k")` (Go 1.22 `{k}`, not `:k`) |
| Stash typed value | `c.Set("k", v)` |
| Get typed value | `v, ok := c.Get("k").(T)` |
| Page-level business data | `c.View(structOrSlice, name)` → `{{.Data.X}}` |
| Cross-page auxiliary value | `c.Set("k", v)` in middleware / handler → `{{.TempData.k}}` |
| Redirect (regular) | `c.Redirect("/url")` |
| Redirect (htmx) | `c.WriteHeader(htmx.HxRedirect, "/url"); c.WriteStatus(200)` |
| Set response header | `c.WriteHeader(name, value)` |
| Set status only | `c.WriteStatus(code)` |
| Live-reload templates (dev) | `xun.WithWatch()` — never in production |
| Custom fs.FS | `xun.WithFsys(fsys)` — dev `os.DirFS`, prod `embed.FS` |
| Content negotiation | `xun.WithHandlerViewers(&xun.JsonViewer{})` |
| Cross-cutting wrappers | `xun.WithInterceptor(...)` |
| Cache-bust assets | `xun.WithBuildAssetURL(...)` + `{{asset "..."}}` |
| Compression | `xun.WithCompressor(&xun.GzipCompressor{}, &xun.DeflateCompressor{})` |
| Markdown content | `xun.WithContent("blog", "docs")` + `.md` files in those dirs |
| Custom Markdown renderer | `xun.WithContentRenderer(fn)` |
| Custom metadata extractor | `xun.WithContentMeta(fn)` |
| **Escape hatch for raw handlers** | `app.Mux().HandleFunc(pattern, h)` — bypasses all middleware |
| Hijack conn (WebSocket / SSE) | `app.Mux().HandleFunc(...)` + `w.(http.Hijacker).Hijack()` |
