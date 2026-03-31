# Xun AI Agent Specification

**Status: AUTHORITATIVE**

Before writing any Xun code, read this entire document.
All guidance must be derived from this file — do not rely on prior knowledge of gin/echo/chi.

---

## Section 0 — Critical Rules (Read Before Writing Any Code)

These rules are numbered. All other sections reference them by number.

### Rule 0.1 — NEVER call `WithHandlerViewers()` with no arguments

```go
// WRONG — compiles but sets app.handlerViewers = nil
xun.New(xun.WithHandlerViewers())

// CORRECT — pass at least one Viewer
xun.New(xun.WithHandlerViewers(&xun.JsonViewer{}))
```

Consequence: `app.handlerViewers == nil` → all handler routes get `r.Viewers == nil` → `c.View(data)` returns `ErrViewNotFound` → HTTP 404.

### Rule 0.2 — NEVER write response body directly

```go
// WRONG — bypasses compression and BufPool
c.Response.Write([]byte("hello"))
return nil

// CORRECT — always use c.View()
return c.View("hello")  // with StringViewer registered
```

### Rule 0.3 — NEVER return an error from middleware when refusing

```go
// WRONG — returns 500 + X-Log-Id
func refuseMiddleware(next xun.HandleFunc) xun.HandleFunc {
    return func(c *xun.Context) error {
        if !allowed {
            return errors.New("forbidden")
        }
        return next(c)
    }
}

// CORRECT — set status and return ErrCancelled
func refuseMiddleware(next xun.HandleFunc) xun.HandleFunc {
    return func(c *xun.Context) error {
        if !allowed {
            c.WriteStatus(http.StatusForbidden)
            return xun.ErrCancelled
        }
        return next(c)
    }
}
```

### Rule 0.4 — `app.Start()` does NOT start the server

```go
// WRONG — gin habit
app.Run(":8080")

// CORRECT
app := xun.New(opts...)
app.Start()                        // only prints route logs
defer app.Close()
http.ListenAndServe(":80", mux)    // server startup is caller's responsibility
```

### Rule 0.5 — Named viewer MUST match Accept header or silently falls back

```go
// Route: r.Viewers = [JsonViewer]
// Request: Accept: application/json

return c.View(user, "views/user/profile")  // views/user/profile is HtmlViewer
// HtmlViewer (text/html) does NOT match Accept (application/json)
// → falls back to JsonViewer (r.Viewers[0]), NOT the named viewer
```

### Rule 0.6 — `pages/*` auto-registers GET only

```go
// File: pages/admin/dashboard.html
// Route: GET /admin/dashboard     ← GET only, no POST/PUT/DELETE auto-registered
// To handle POST, register explicitly:
app.Post("/admin/dashboard", handler)
```

### Rule 0.7 — `{$}` means trailing slash required

```go
app.Get("/posts/{$}")   // matches GET /posts/  ONLY
app.Get("/posts/")       // matches GET /posts/abc, GET /posts/123
app.Get("/posts")        // matches GET /posts  ONLY (no slash)
```

---

## Section 1 — Types

```
HandleFunc       = func(c *Context) error
Middleware       = func(next HandleFunc) HandleFunc
Option           = func(*App)
RoutingOption    = func(*RoutingOptions)
chain            = interface{ Next(hf HandleFunc) HandleFunc }
```

`HandleFunc` returns `error`, not `nil`. See Section 10 for error meanings.

---

## Section 2 — App

### 2.1 Creation

```go
app := xun.New(opts ...Option) *App
```

### 2.2 Fields

| Field | Type | Default | Overridden-By | Nil-Result |
|-------|------|---------|---------------|------------|
| `app.mux` | `*http.ServeMux` | `http.DefaultServeMux` | `WithMux(mux)` | — |
| `app.handlerViewers` | `[]Viewer` | `[]Viewer{&JsonViewer{}}` | `WithHandlerViewers(v...)` | All handler routes return 404 (Rule 0.1) |
| `app.fsys` | `fs.FS` | `nil` | `WithFsys(fsys)` | Page routing disabled |
| `app.watch` | `bool` | `false` | `WithWatch()` | Hot reload disabled |
| `app.interceptor` | `Interceptor` | `nil` | `WithInterceptor(i)` | Redirect/RequestReferer use defaults |
| `app.compressors` | `[]Compressor` | `nil` | `WithCompressor(c...)` | No compression |
| `app.viewers` | `map[string]Viewer` | `empty map` | `HtmlViewEngine.Load()` registers `views/*` | Named viewers unavailable |
| `app.funcMap` | `template.FuncMap` | `xun.builtins` | `WithTemplateFunc`, `WithTemplateFuncMap` | Builtin `asset` func unavailable |
| `app.routes` | `map[string]*Routing` | `empty map` | `app.Get/Post/etc`, `app.HandlePage` | — |

### 2.3 App.Start()

```go
app.Start()
```

Writes info-level logs for each registered route (pattern + viewer MIME types). Does NOT start the HTTP server. Server startup is the caller's responsibility.

### 2.4 App.Close()

Currently a no-op. Reserved for future use.

### 2.5 Option Functions

```
WithMux(mux *http.ServeMux) Option
WithFsys(fsys fs.FS) Option
WithWatch() Option                    // dev only — not thread-safe
WithHandlerViewers(v ...Viewer) Option
WithViewEngines(ve ...ViewEngine) Option
WithInterceptor(i Interceptor) Option
WithCompressor(c ...Compressor) Option
WithTemplateFunc(name string, fn any) Option
WithTemplateFuncMap(fm template.FuncMap) Option
WithBuildAssetURL(match func(string) bool) Option
WithLogger(logger *slog.Logger) Option
```

### 2.6 Route Registration

```
app.Get(pattern string, hf HandleFunc, opts ...RoutingOption)
app.Post(pattern string, hf HandleFunc, opts ...RoutingOption)
app.Put(pattern string, hf HandleFunc, opts ...RoutingOption)
app.Delete(pattern string, hf HandleFunc, opts ...RoutingOption)
app.Group(prefix string) *group
```

Pattern format: `"METHOD pattern"` (e.g., `"GET /users/{id}"`). Go 1.22 ServeMux syntax.

---

## Section 3 — Group

`group` implements `chain`.

```go
func (g *group) Use(middleware ...Middleware)
func (g *group) Get(pattern string, hf HandleFunc, opts ...RoutingOption)
func (g *group) HandleFunc(pattern string, hf HandleFunc, opts ...RoutingOption)
func (g *group) Next(hf HandleFunc) HandleFunc
```

Middleware chain construction (inside-out):

```go
// given [A, B, C] and handler H:
// build: C(B(A(H)))
next := H
for i := len(g.middlewares); i > 0; i-- {
    next = g.middlewares[i-1](next)
}
```

---

## Section 4 — Middleware

Middleware signature: `func(next HandleFunc) HandleFunc`

```go
func AuthMiddleware(next xun.HandleFunc) xun.HandleFunc {
    return func(c *xun.Context) error {
        // pre logic
        token := c.Request.Header.Get("X-Token")
        if token == "" {
            c.WriteStatus(http.StatusUnauthorized)
            return xun.ErrCancelled
        }
        err := next(c)
        // post logic (runs after handler)
        return err
    }
}
```

Pre-logic: runs before `next(c)`. Post-logic: runs after `next(c)` returns.
On refusal: ALWAYS set status + return `xun.ErrCancelled` (Rule 0.3).

---

## Section 5 — Context

`Context` wraps `*http.Request`, `ResponseWriter`, and application state.

### 5.1 Fields

```
c.Request  *http.Request    // standard library
c.Response ResponseWriter   // xun interface (extends http.ResponseWriter)
c.Routing  Routing          // route metadata
c.App      *App            // application instance
c.TempData TempData        // map[string]any, request-scoped storage
```

### 5.2 Standard Library Equivalents

Use standard library directly for these:

```
c.Request.PathValue("id")              // path parameter (Go 1.22+)
c.Request.URL.Query().Get("name")     // query string
c.Request.Header.Get("X-Token")       // headers
c.Request.Cookie("session_id")        // read cookie
c.Request.Body                         // request body
c.Request.ParseMultipartForm()         // multipart form
c.Request.Context()                    // context.Context
c.Response.Header().Set(k, v)         // response headers
http.SetCookie(c.Response, &cookie)   // write cookie
c.Request.RemoteAddr                   // client address (no proxy support; use ext/proxyproto)
```

### 5.3 xun-Specific Methods

```
c.View(data any, options ...string) error
c.Redirect(url string, statusCode ...int)
c.AcceptLanguage() []string
c.Accept() []MimeType
c.RequestReferer() string
c.WriteStatus(code int)
c.WriteHeader(key string, value string)
c.Get(key string) any
c.Set(key string, value any)
```

### 5.4 c.View(data any, options ...string) Behavior

```
IF options[0] is provided (named viewer name):
  → getViewer(name) checks: named viewer.MimeType() matches any Accept header
  → IF match: use named viewer
  → IF no match: proceed to step 2

ELSE skip to step 2.

STEP 2: Iterate Accept headers, match against r.Viewers:
  → First matching viewer is used

STEP 3: No match found:
  → Use r.Viewers[0] as fallback

STEP 4: r.Viewers is empty at this point:
  → Return ErrViewNotFound → HTTP 404
```

`c.View()` sets status 200 automatically. Call `c.WriteStatus()` before `c.View()` to override.

### 5.5 c.Redirect(url string, statusCode ...int)

Sets `Location` header. Default status: `http.StatusFound` (302). Interceptor can override if configured.

---

## Section 6 — Routing

### 6.1 Routing Fields

```
type Routing struct {
    Pattern string
    Handle  HandleFunc
    chain   chain           // *App or *group
    Options *RoutingOptions
    Viewers []Viewer       // viewer's for this route
}
```

### 6.2 Routing.Next(ctx)

```go
func (r *Routing) Next(ctx *Context) error {
    return r.chain.Next(r.Handle)(ctx)
}
```

### 6.3 RoutingOptions Fields

```
type RoutingOptions struct {
    metadata map[string]any
    viewers  []Viewer
}
```

### 6.4 RoutingOption Functions

```
WithViewer(v ...Viewer) RoutingOption
WithMetadata(key string, value any) RoutingOption
WithNavigation(name, icon, access string) RoutingOption
```

---

## Section 7 — Viewer

### 7.1 Interface

```go
type Viewer interface {
    MimeType() *MimeType
    Render(ctx *Context, data any) error
}
```

### 7.2 Built-in Viewers

| Viewer | MimeType | Default For |
|--------|----------|------------|
| `HtmlViewer` | `text/html` | Page routes |
| `JsonViewer` | `application/json` | Handler routes (only if `app.handlerViewers` not overridden) |
| `TextViewer` | `text/*` (from filename) | Text templates |
| `XmlViewer` | `text/xml` | — |
| `StringViewer` | `text/plain` | — |
| `FileViewer` | `*/*` | Static files |

### 7.3 Implementing a Viewer

```go
type MyViewer struct{}

func (*MyViewer) MimeType() *xun.MimeType {
    return &xun.MimeType{Type: "application", SubType: "json"}
}

func (*MyViewer) Render(c *xun.Context, data any) error {
    c.Response.Header().Set("Content-Type", "application/json")
    buf := xun.BufPool.Get()
    defer xun.BufPool.Put(buf)
    // render JSON to buf
    json.NewEncoder(buf).Encode(data)
    _, err := buf.WriteTo(c.Response)
    return err
}
```

`BufPool` = `*xun.BufferPool` (pool of `*bytes.Buffer`). `Get()` returns a buffer; `Put(buf)` returns it. Always `defer Put(buf)` immediately after `Get()`.

### 7.4 Named Viewers

Named viewers are stored in `app.viewers` (map[string]Viewer). They are registered automatically by `HtmlViewEngine.Load()` for files under `views/`.

Registration: `app.viewers["views/user/profile"] = &HtmlViewer{template: t}`

Usage: `return c.View(user, "views/user/profile")` (Rule 0.5 applies).

---

## Section 8 — ViewEngine

### 8.1 Interface

```go
type ViewEngine interface {
    Load(fsys fs.FS, app *App)
    FileChanged(fsys fs.FS, app *App, event fsnotify.Event) error
}
```

### 8.2 Built-in Engines

| Engine | Loads | Hot Reload Triggers |
|--------|-------|---------------------|
| `StaticViewEngine` | `public/*` as routes | `public/*` Create/Write |
| `HtmlViewEngine` | `components/`, `layouts/`, `pages/`, `views/` | `*.html` Create/Write in those dirs |
| `TextViewEngine` | `text/*` | `text/*` Create/Write |

Default engines loaded when `app.engines == nil` (i.e., `New()` called without `WithViewEngines`).

### 8.3 HtmlViewEngine Dependency Graph

When a layout is reloaded, HtmlViewEngine tracks dependents and reloads all pages that `{{ define }}` blocks from that layout.

---

## Section 9 — Project Structure

```
public/      → Static assets. public/index.html → GET /{$}
components/  → Reusable HTML fragments. Include via {{ block "components/name" . }}
layouts/     → Page layouts. Select with <!--layout:name--> at top of page file.
pages/       → Auto-routed pages. pages/foo/index.html → GET /foo/{$}
              pages only registers GET. Other methods require explicit handlers.
views/       → Named views (not auto-routed). Use via c.View(data, "views/name")
text/        → text/template files. c.View(data, "text/sitemap.xml")
```

Dynamic segments: `{var}` in filenames → e.g., `pages/user/{id}.html` → GET `/user/{id}`.
Multiple hosts: `@host.com/` prefix → e.g., `pages/@abc.com/index.html` → GET `abc.com/{$}`.

### 9.1 Layout Selection

File: `pages/user/profile.html`
```
<!--layout:admin-->
{{ define "content" }}
  <p>{{ .Data.Name }}</p>
{{ end }}
```

Layout file: `layouts/admin.html`
```
<!DOCTYPE html>
<html>
<body>
{{ block "content" . }}{{ end }}
</body>
</html>
```

### 9.2 Template Model

`ViewModel{TempData, Data}` is passed to templates. Access via `.Data` and `.TempData`.

```html
<p>Name: {{ .Data.Name }}</p>
{{ if .TempData.Session }}
  <p>Session: {{ .TempData.Session }}</p>
{{ end }}
```

---

## Section 10 — Error Handling

| Return Value | Framework Behavior |
|-------------|-------------------|
| `return nil` | Response complete |
| `return xun.ErrCancelled` | Stop middleware chain; response already handled |
| `return xun.ErrViewNotFound` | Emit 404 |
| `return other error` | Emit 500 + X-Log-Id header |

`ErrCancelled` usage (Rule 0.3): after calling `c.WriteStatus()` to set the status.

---

## Section 11 — Static Assets and Fingerprinting

### 11.1 Static Files

`StaticViewEngine.Load()` registers all non-dir files under `public/` as routes.

### 11.2 Fingerprinting

```go
app := xun.New(
    xun.WithFsys(fsys),
    xun.WithBuildAssetURL(func(path string) bool {
        return strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css")
    }),
)
```

Flow:
1. Matcher returns true for `/assets/app.js`
2. StaticViewEngine computes content ETag
3. Registers `/assets/app-a1b2c3.js` as separate route
4. `app.AssetURLs["/assets/app.js"] = "/assets/app-a1b2c3.js"`
5. `{{ asset "/assets/app.js" }}` in templates returns `/assets/app-a1b2c3.js`

Fingerprinted assets get `Cache-Control: public, max-age=31536000, immutable`.

---

## Section 12 — Compression

### 12.1 Compressors

```
WithCompressor(&xun.GzipCompressor{})
WithCompressor(&xun.DeflateCompressor{})
```

Selected by `Accept-Encoding` header. `*` in Accept-Encoding matches all.

### 12.2 ResponseWriter Interface

```go
type ResponseWriter interface {
    http.ResponseWriter
    BodyBytesSent() int
    StatusCode() int
    Close()
}
```

`Close()` is called automatically by framework via defer in handler wrapper. Do not call manually.

---

## Section 13 — Redirects and Interceptor

### 13.1 Redirect

```go
c.Redirect(url)              // 302 by default
c.Redirect(url, 301)         // custom status
```

### 13.2 Interceptor Interface

```go
type Interceptor interface {
    RequestReferer(c *Context) string
    Redirect(c *Context, url string, statusCode ...int) bool
}
```

If `Redirect` returns true, the default redirect behavior is skipped.
Use `WithInterceptor(htmx.New())` for htmx support.

---

## Section 14 — Form Binding and Validation (ext/form)

### 14.1 Binding Functions

```
form.BindQuery[T any](req *http.Request) (*TEntity[T], error)
form.BindForm[T any](req *http.Request) (*TEntity[T], error)
form.BindJson[T any](req *http.Request) (*TEntity[T], error)
```

### 14.2 TEntity

```go
type TEntity[T any] struct {
    Data   T                 `json:"data"`
    Errors map[string]string `json:"errors"`
}
```

`it.Validate(languages...)` populates `Errors` and returns false if validation fails.

### 14.3 Validation Setup

```go
import (
    "github.com/go-playground/locales/zh"
    ut "github.com/go-playground/universal-translator"
    trans "github.com/go-playground/validator/v10/translations/zh"
    "github.com/yaitoo/xun"
    "github.com/yaitoo/xun/ext/form"
)

xun.AddValidator(ut.New(zh.New()).GetFallback(), trans.RegisterDefaultTranslations)
```

Call before registering routes.

### 14.4 Complete Form Handler

```go
type Login struct {
    Email  string `form:"email" json:"email" validate:"required,email"`
    Passwd string `json:"passwd" validate:"required"`
}

app.Post("/login", func(c *xun.Context) error {
    it, err := form.BindForm[Login](c.Request)
    if err != nil {
        c.WriteStatus(http.StatusBadRequest)
        return xun.ErrCancelled
    }
    if !it.Validate(c.AcceptLanguage()...) {
        c.WriteStatus(http.StatusBadRequest)
        return c.View(it)
    }
    // process login
    return c.Redirect("/dashboard")
})
```

---

## Section 15 — Extensions

### 15.1 Extensions Summary

| Extension | Import | Registration | Key Functions |
|-----------|--------|--------------|---------------|
| `acl` | `ext/acl` | `app.Use(acl.New(...))` | AllowHosts, AllowIPNets, DenyCountries |
| `autotls` | `ext/autotls` | `autotls.New(...).Configure(srv, srvTLS)` | New, WithCache, WithHosts, Configure |
| `cache` | `ext/cache` | `cache.New()` | Get, Set, Delete |
| `cookie` | `ext/cookie` | — (stateless) | Set, Get, SetSigned, GetSigned, Delete |
| `csrf` | `ext/csrf` | `app.Use(csrf.New(secret))` | New, WithJsToken, HandleFunc |
| `form` | `ext/form` | — | BindQuery, BindForm, BindJson |
| `hsts` | `ext/hsts` | `app.Use(hsts.WriteHeader())` | Redirect, WriteHeader |
| `htmx` | `ext/htmx` | `xun.WithInterceptor(htmx.New())` | New |
| `proxyproto` | `ext/proxyproto` | `proxyproto.ListenAndServe(srv)` | ListenAndServe, ListenAndServeTLS |
| `reqlog` | `ext/reqlog` | `app.Use(reqlog.New(...))` | New, WithFormat, WithLogger |
| `sse` | `ext/sse` | `ss := sse.New()` | New, Join, Send, Broadcast, Leave, Shutdown |

### 15.2 Cookie Extension

```go
import "github.com/yaitoo/xun/ext/cookie"

// Base64 encoded (not signed)
cookie.Set(c, http.Cookie{Name: "theme", Value: "dark"})
v, err := cookie.Get(c, "theme")

// HMAC signed
ts, err := cookie.SetSigned(c, http.Cookie{Name: "session", Value: "abc123"}, []byte("secret"))
v, ts, err := cookie.GetSigned(c, "session", []byte("secret"))

// Delete
cookie.Delete(c, http.Cookie{Name: "theme"})
```

---

## Section 16 — Performance

- Do NOT enable `WithWatch()` in production (not thread-safe).
- Use `xun.BufPool` in custom Viewer implementations to reduce allocations.
- Compressors create per-request writers. Always rely on framework's deferred `Close()`.
- `app.Start()` does not start the server (Rule 0.4).

---

## Section 17 — Go 1.22 Router Syntax

xun uses Go's built-in `http.ServeMux` router from Go 1.22.

```
GET /users              matches /users
GET /users/             matches /users, /users/, /users/123
GET /users/{id}         matches /users/123, sets PathValue("id") = "123"
GET /users/{id}/posts   matches /users/123/posts
GET /posts/{$}          matches /posts/ ONLY (trailing slash required)
```

---

## Section 18 — Gin/Echo/Chi Differences

### 18.1 What xun Does NOT Have

These do NOT exist on `*xun.Context`:

```
c.Query("name")              → c.Request.URL.Query().Get("name")
c.PostForm("email")          → form.BindForm[T](c.Request)
c.Cookie("name")            → c.Request.Cookie("name")
c.SetCookie(name, v, ...)   → http.SetCookie(c.Response, &http.Cookie{...})
c.JSON(200, data)           → c.View(data) (with JsonViewer)
c.HTML(200, "tpl", data)   → c.View(data, "views/tpl") (with HtmlViewer)
c.String(200, "ok")        → c.View("ok") (with StringViewer)
c.Data(200, mime, buf)     → c.View(buf) (with FileViewer)
c.Bind(&user)              → form.BindJson[T](c.Request), form.BindForm[T](c.Request)
c.ShouldBind(&user)        → same as above
c.FullPath()               → does not exist
c.HandlerName()             → does not exist
c.MustGet("key")           → c.Get("key") (returns nil if missing, no panic)
c.Abort()                  → does not exist
c.AbortWithStatusJSON(...) → does not exist
c.ClientIP()               → c.Request.RemoteAddr (no built-in proxy support)
```

### 18.2 Middleware Signature Difference

```go
// Gin: c.Next() called INSIDE handler
func ginMiddleware(c *gin.Context) {
    // pre
    c.Next()
    // post
}

// xun: next() called EXPLICITLY, returns HandleFunc
func xunMiddleware(next xun.HandleFunc) xun.HandleFunc {
    return func(c *xun.Context) error {
        // pre
        err := next(c)
        // post
        return err
    }
}
```

### 18.3 Error Handling Difference

```go
// Gin: refusing returns nothing, response handled
if !allowed {
    c.AbortWithStatus(http.StatusUnauthorized)
    return
}

// xun: refusing sets status and returns ErrCancelled
if !allowed {
    c.WriteStatus(http.StatusUnauthorized)
    return xun.ErrCancelled
}
```

---

## Section 19 — Complete Minimal Examples

### 19.1 Minimal JSON API (No fs.FS)

```go
package main

import (
    "net/http"

    "github.com/yaitoo/xun"
)

func main() {
    app := xun.New(
        xun.WithHandlerViewers(&xun.JsonViewer{}), // Rule 0.1
    )

    app.Get("/ping", func(c *xun.Context) error {
        return c.View(map[string]string{"message": "pong"})
    })

    app.Start()
    defer app.Close()
    http.ListenAndServe(":8080", http.DefaultServeMux)
}
```

### 19.2 JSON + HTML Same Route

```go
app := xun.New(
    xun.WithHandlerViewers(&xun.JsonViewer{}, &xun.HtmlViewer{}),
)

app.Get("/user/{id}", func(c *xun.Context) error {
    id := c.Request.PathValue("id")
    return c.View(getUser(id))
})
// Accept: application/json → JsonViewer
// Accept: text/html → HtmlViewer
```

### 19.3 Production with embed.FS

```go
//go:embed app
var fsys embed.FS

func main() {
    var dev bool
    flag.BoolVar(&dev, "dev", false, "dev")
    flag.Parse()

    var opts []xun.Option
    if dev {
        opts = []xun.Option{
            xun.WithFsys(os.DirFS("./app")),
            xun.WithWatch(),
            xun.WithHandlerViewers(&xun.HtmlViewer{}),
        }
    } else {
        sub, _ := fs.Sub(fsys, "app")
        opts = []xun.Option{
            xun.WithFsys(sub),
            xun.WithHandlerViewers(&xun.HtmlViewer{}),
        }
    }

    app := xun.New(opts...)
    app.Get("/{$}", func(c *xun.Context) error {
        return c.View(map[string]string{"hello": "xun"})
    })

    app.Start()
    defer app.Close()
    http.ListenAndServe(":80", http.DefaultServeMux)
}
```

### 19.4 Handler with Middleware Group

```go
auth := app.Group("/admin")
auth.Use(func(next xun.HandleFunc) xun.HandleFunc {
    return func(c *xun.Context) error {
        cookie, err := c.Request.Cookie("session")
        if err != nil || cookie.Value == "" {
            c.Redirect("/login?return=" + c.Request.URL.String())
            return xun.ErrCancelled
        }
        c.Set("Session", cookie.Value)
        return next(c)
    }
})

auth.Get("/{$}", func(c *xun.Context) error {
    return c.View(map[string]any{"user": c.Get("Session")})
})
```

---

## Section 20 — Quick Reference

### Rule Index

```
Rule 0.1 — WithHandlerViewers() requires at least one argument
Rule 0.2 — Never write c.Response directly — always use c.View()
Rule 0.3 — On refusal: c.WriteStatus() + return ErrCancelled, never return error
Rule 0.4 — app.Start() does not start server
Rule 0.5 — Named viewer must match Accept header
Rule 0.6 — pages/* registers GET only
Rule 0.7 — {$} means trailing slash required
```

### Section Index

```
Section 0  — Critical Rules
Section 1  — Types
Section 2  — App (creation, fields, options)
Section 3  — Group
Section 4  — Middleware
Section 5  — Context
Section 6  — Routing
Section 7  — Viewer
Section 8  — ViewEngine
Section 9  — Project Structure
Section 10 — Error Handling
Section 11 — Static Assets and Fingerprinting
Section 12 — Compression
Section 13 — Redirects and Interceptor
Section 14 — Form Binding and Validation
Section 15 — Extensions
Section 16 — Performance
Section 17 — Go 1.22 Router Syntax
Section 18 — Gin/Echo/Chi Differences
Section 19 — Complete Minimal Examples
Section 20 — Quick Reference
```
