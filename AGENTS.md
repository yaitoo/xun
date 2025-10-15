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
```go
app := xun.New(
  xun.WithMux(http.DefaultServeMux),
  xun.WithFsys(viewsFS),          // fs root of components/layouts/pages/views/public/text
  xun.WithWatch(),                // dev only
  xun.WithCompressor(&xun.GzipCompressor{}, &xun.DeflateCompressor{}),
)
app.Start()
defer app.Close()
http.ListenAndServe(":80", http.DefaultServeMux)
```
Notes: WithWatch is not thread-safe; enable only in development. In production embed assets and disable Watch.

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

## 7) Templates and data model
- HTML uses html/template; text uses text/template (no HTML auto-escaping, ensure safety).
- Layout selection: put `<!--layout:home-->` at page top for layouts/home.html.
- Template model: Viewer receives ViewModel{TempData, Data}; use `.Data` and `.TempData` in templates.
- Template funcs: register via WithTemplateFunc/WithTemplateFuncMap. In production, built-in `asset` resolves fingerprinted asset URLs.

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
- Context.View with named viewer must still match Accept; otherwise negotiation iterates route viewers, then first as fallback.
- text/* uses text/template (no HTML escaping) – sanitize output as needed.
- FileViewer supports ETag/If-None-Match; when using embed FS, ETag is auto-computed.

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
