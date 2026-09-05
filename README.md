# Xun
Xun is a web framework built on Go's built-in html/template and net/http package’s router. It is designed to be lightweight, fast, and easy to use. Xun provides a simple and intuitive API for building web applications, while also offering advanced features such as middleware, routing, and template rendering.

Xun [ʃʊn] (pronounced 'shoon'), derived from the Chinese character 迅, signifies being lightweight and fast.

[![Tests](https://github.com/yaitoo/xun/actions/workflows/tests.yml/badge.svg)](https://github.com/yaitoo/xun/actions/workflows/tests.yml)
[![Codecov](https://codecov.io/gh/yaitoo/xun/branch/main/graph/badge.svg)](https://codecov.io/gh/yaitoo/xun)
[![Go Report Card](https://goreportcard.com/badge/github.com/yaitoo/xun)](https://goreportcard.com/report/github.com/yaitoo/xun)
[![Go Reference](https://pkg.go.dev/badge/github.com/yaitoo/xun.svg)](https://pkg.go.dev/github.com/yaitoo/xun)
[![GitHub Release](https://img.shields.io/github/v/release/yaitoo/xun)](https://github.com/yaitoo/xun/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-blue.svg)](https://github.com/yaitoo/xun/compare)

## Features
- Works with Go's built-in `net/http.ServeMux` router that was introduced in 1.22. [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements).
- Works with Go's built-in `html/template`. It is built-in support for Server-Side Rendering (SSR).
- Built-in response compression support for `gzip` and `deflate`.
- Built-in Form and Validate feature with i18n support.
- Built-in `AutoTLS` feature. It automatic SSL certificate issuance and renewal through Let's Encrypt and other ACME-based CAs
- Support Page Router in `StaticViewEngine` and `HtmlViewEngine`.
- Support multiple viewers by ViewEngines: `StaticViewEngine`, `JsonViewEngine` and `HtmlViewEngine`. You can feel free to add custom view engine, eg `XmlViewEngine`.
- Support to reload changed static files automatically in development environment.
- Built-in Content engine that renders `.md` files in `content/` as pages, deriving `Title` from the first `# H1` and `Description` from the first blockquote or top-level paragraph — no frontmatter, no separate template pipeline.



## Getting Started
> See full source code on [xun-examples](https://github.com/yaitoo/xun-examples)

### Install Xun
- install latest commit from `main` branch
```
go get github.com/yaitoo/xun@main
```

- install latest release
```
go get github.com/yaitoo/xun@latest
```

### Project structure
`Xun` has some specified directories that is used to organize code, routing and static assets.
- `public`: Static assets to be served.
- `components` A partial view that is shared between layouts/pages/views.
- `views`: An internal page view that can be referenced in `context.View` to render different UI for current routing.
- `layouts`: A layout is shared between multiple pages/views
- `pages`: A public page view that will create public page routing automatically.
- `text`: An internal text view that can be referenced in `context.View` to render with a data model.
- `content`: A Markdown-driven page tree. Every `.md` file auto-registers as `GET /<slug>`; paired `.tpl` files act as the bubble-up template (never registered as a route) and use the same `layouts/` directory as everything else. `.html` files in `content/` are independent page routes.

**NOTE: All html files(component,layout, view and page) will be parsed by [html/template](https://pkg.go.dev/html/template). You can feel free to use all built-in [Actions,Pipelines and Functions](https://pkg.go.dev/text/template), and your custom functions that is registered in `HtmlViewEngine`.**

### Layouts and Pages
`Xun` uses file-system based routing, meaning you can use folders and files to define routes. This section will guide you through how to create layouts and pages, and link between them.


#### Creating a page
A page is UI that is rendered on a specific route. To create a page, add a page file(.html) inside the `pages` directory. For example, to create an index page (`/`):
```
└── app
    └── pages
        └── index.html
```

> index.html
``` html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Xun-Admin</title>
  </head>
  <body>
    <div id="app">hello world</div>
  </body>
</html>
```

#### Creating a layout
A layout is UI that is shared between multiple pages/views.

You can create a layout(.html) file inside the `layouts` directory.
```
└── app
    ├── layouts
    │   └── home.html
    └── pages
        └── index.html
```

> layouts/home.html
```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Xun-Admin</title>
  </head>
  <body>
    {{ block "content" .}} {{ end }}
  </body>
</html>
```
> pages/index.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app">hello world</div>
{{ end }}
```

### Static assets
You can store static files, like images, fonts, js and css, under a directory called `public` in the root directory. Files inside public can then be referenced by your code starting from the base URL (/).

**NOTE: `public/index.html` will be exposed by `/` instead of `/index.html`.**

#### Creating a component
A component is a partial view that is shared between multiple layouts/pages/views.

```
└── app
    ├── components
    │   └── assets.html
    ├── layouts
    │   └── home.html
    ├── pages
    │   └── index.html
    └── public
        ├── app.js
        └── skin.css
```
> components/assets.html
```html
<link rel="stylesheet" href="/skin.css">
<script type="text/javascript" src="/app.js"></script>
```
> layouts/home.html
```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Xun-Admin</title>
    {{ block "components/assets" . }} {{ end }}
  </head>
  <body>
    {{ block "content" .}} {{ end }}
  </body>
</html>
```

### Text View
A text view is UI that is referenced in `context.View` to render the view with a data model.

**NOTE: Text files are parsed using the `text/template` package. This is different from the `html/template` package used in `pages/layouts/views/components`. While `text/template` is designed for generating textual output based on data, it does not automatically secure HTML output against certain attacks. Therefore, please ensure your output is safe to prevent code injection.**

#### Creating a text view
```
└── app
    ├── components
    │   └── assets.html
    ├── layouts
    │   └── home.html
    ├── pages
    │   └── index.html
    └── public
    │   ├── app.js
    │   └── skin.css
    └── text
        ├── sitemap.xml
```

#### Render the view with a data model
```go
	app.Get("/sitemap.xml", func(c *xun.Context) error {
		return c.View(Sitemap{
			LastMod: time.Now(),
		}, "text/sitemap.xml") // use `text/sitemap.xml` as current Viewer to render
	})
```

> curl --header "Accept: application/xml, text/xml,text/plain, */*" -v http://127.0.0.1/sitemap.xml

```bash
*   Trying 127.0.0.1:80...
* Connected to 127.0.0.1 (127.0.0.1) port 80
> GET /sitemap.xml HTTP/1.1
> Host: 127.0.0.1
> User-Agent: curl/8.7.1
> Accept: application/xml, text/xml,text/plain, */*
>
* Request completely sent off
< HTTP/1.1 200 OK
< Date: Wed, 15 Jan 2025 11:51:56 GMT
< Content-Length: 277
< Content-Type: text/xml; charset=utf-8
<
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
  <loc>https://github.com/yaitoo/xun</loc>
  <lastmod>2025-01-15T19:51:56+08:00</lastmod>
  <changefreq>hourly</changefreq>
  <priority>1.0</priority>
  </url>
* Connection #0 to host 127.0.0.1 left intact
</urlset>%
```



### Content
The Content engine turns Markdown files into pages without introducing a new `Viewer` or a second template engine. Drop a `.md` file into `content/` and it auto-registers as `GET /<slug>`; drop a `.tpl` next to it (or any ancestor `index.tpl`) to control how the rendered Markdown is wrapped.

`.html` and `.tpl` inside `content/` have distinct roles:

- `.tpl` — bubble-up template only. Loaded into the template graph; never registered as a route.
- `.html` — page route only. Registered as a route; never consulted during bubble-up.

This is what lets a directory such as `content/blog/` hold both `index.tpl` (wrapping template) and `index.md` (real Markdown page) without conflict.

#### Directory layout
```
└── app
    ├── layouts
    │   └── site.html
    └── content
        ├── hello.md           ← GET /hello
        ├── hello.tpl          ← optional, dedicated template for /hello
        └── 2026
            ├── index.tpl      ← bubble-up target (not a route)
            ├── index.md       ← optional, directory-level page: GET /2026/
            └── deeper.md      ← GET /2026/deeper
```

For each `.md` file, the engine looks for a bubble-up template in this order:

1. `content/<slug>.tpl`
2. `content/<dir>/index.tpl`
3. any ancestor `index.tpl`
4. the root `index.tpl`

The first match is the template; it uses the same `<!--layout:NAME-->` mechanism as every other page, so layouts are shared across content and non-content pages.

#### Writing a Markdown file
Markdown files don't need any frontmatter. `Title` is taken from the first `# H1` and `Description` from the first blockquote (preferred) or top-level paragraph. Everything else comes from the filesystem path and `mtime`.

> content/hello.md
```markdown
# Hello, Xun

> A one-line pitch for the page.

This is the body. It supports standard [GitHub Flavored
Markdown](https://github.github.com/gfm/) out of the box — tables,
task lists, fenced code, autolinks.
```

#### Rendering the page in a template
A request to `/hello` lands on `content/hello.tpl` (or whichever template the bubble-up rule picked). Inside the template, the parsed Markdown is exposed as `.Content`:

> content/hello.tpl
```html
<!--layout:site-->
{{ define "content" }}
  <article>
    <h1>{{ .Content.Title }}</h1>
    {{ if .Content.Description }}
      <p class="lead">{{ .Content.Description }}</p>
    {{ end }}
    <div>{{ .Content.Body }}</div>
  </article>
{{ end }}
```

`Content` is `nil` on routes that don't come from a `.md` file, so existing templates are unaffected. The full shape:

```go
type ContentView struct {
    Path        string         // "content/2026/deeper.md"
    Slug        string         // "2026/deeper"
    Title       string         // First # H1; empty if none
    Description string         // First blockquote (preferred) or top-level paragraph
    Date        time.Time      // File mtime
    Body        template.HTML  // Rendered Markdown
}
```

#### Configuring the engine
`content/` is enabled by default. The relevant options are:

- `xun.WithContent(dirs ...string)` — register one or more content directories (default `["content"]`). Passing `""` disables Markdown loading. Each directory doubles as a route prefix, so `WithContent("blog", "docs")` produces independent `/blog/...` and `/docs/...` trees.
- `xun.WithContentRenderer(fn)` — replace the default Markdown renderer. Use this to plug in syntax highlighting, AST transforms, or a fully-configured `goldmark` instance.

Custom key/value metadata (image, author, OG tags, etc.) is read from a sibling `.yaml` next to each `.md`. See `docs/content.md` §1.4 for the full convention.

#### A minimal example
```go
app := xun.New(
    xun.WithFsys(os.DirFS("app")),
    xun.WithContent("content"),   // optional — "content" is the default
)
app.Start()
defer app.Close()
http.ListenAndServe(":80", http.DefaultServeMux)
```

With `content/hello.md` and `content/hello.tpl` in place, `curl http://localhost/hello` returns the rendered page.



## Building your application
### Canonical application shape
The standard layout of a `main.go` is a six-step lifecycle — load config, open resources, build the app, register routes, start the app, run listeners. Keeping the steps in this shape makes the program easy to extend and easy to reason about.

```go
// 1. Load config (viper, env, flags — not xun-specific but done first)
err := loadConfig()

// 2. Open databases / caches
db, err := setupDB(ctx)

// 3. Build the xun.App with options
mux := http.NewServeMux()
app := xun.New(
    xun.WithMux(mux),
    xun.WithFsys(getFsys()),
    xun.WithWatch(),                          // dev only
    xun.WithHandlerViewers(&xun.JsonViewer{}),
    xun.WithInterceptor(htmx.New()),
    xun.WithBuildAssetURL(func(p string) bool {
        return strings.HasPrefix(p, "/assets/")
    }),
)

// 4. Register xun-managed routes (go through middleware pipeline)
setupRoutes(app)

// 5. Start the app (non-blocking — registers handlers on the mux)
app.Start()
defer app.Close()

// 6. Run listeners (blocking)
http.ListenAndServe(":80", mux)
```

| Option | Purpose | When to use it |
|---|---|---|
| `WithMux` | Inject your own `*http.ServeMux` | When you need a stdlib mux (e.g. for `ListenAndServeTLS`) |
| `WithFsys` | Templates + assets `fs.FS` | Always — points to `app/` in dev, `//go:embed` in prod |
| `WithWatch` | Live-reload templates | **Dev only**; thread-unsafe |
| `WithHandlerViewers` | Alternate content renderers | Add `JsonViewer` for API responses alongside `HtmlViewer` |
| `WithInterceptor` | Cross-cutting wrappers | `htmx.New()` (request/response helpers), custom auth, etc. |
| `WithBuildAssetURL` | Asset URL strategy | Return `true` to route through `{{asset "..."}}` for cache-bust hashing |

#### Dev / prod twin fs.FS
The canonical pattern keeps templates and assets addressable in both environments:

```go
//go:embed app/components app/layouts app/pages app/views app/public
var fsys embed.FS

func getFsys() fs.FS {
    if fi, err := os.Stat("./app"); err == nil && fi.IsDir() {
        return os.DirFS("./app")            // dev: live files on disk
    }
    sub, _ := fs.Sub(fsys, "app")            // prod: bundled binary
    return sub
}
```

In production the binary has no on-disk dependencies; in development `WithWatch` reloads templates and assets as you save them.

### Routing
#### Route Handler
Page Router only serve static content from html files. We have to define router handler in go to process request and bind data to the template file via `HtmlViewer`.

> pages/index.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app">hello {{.Data.Name}}</div>
{{ end }}
```

> main.go
```go
	app.Get("/{$}", func(c *xun.Context) error {
		return c.View(map[string]string{
			"Name": "go-xun",
		})
	})
```


**NOTE: An `/index.html` always be registered as `/{$}` in routing table. See more detail on [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements).**
> There is one last bit of syntax. As we showed above, patterns ending in a slash, like /posts/, match all paths beginning with that string. To match only the path with the trailing slash, you can write /posts/{$}. That will match /posts/ but not /posts or /posts/234.

#### Dynamic Routes
When you don't know the exact segment names ahead of time and want to create routes from dynamic data, you can use Dynamic Segments that are filled in at request time. `{var}` can be used in folder name and file name as same as router handler in `http.ServeMux`.

For examples, below patterns will be generated automatically, and registered in routing table.
- `/user/{id}.html` generates pattern `/user/{id}`
- `/{id}/user.html` generates pattern `/{id}/user`

```
├── app
│   ├── components
│   │   └── assets.html
│   ├── layouts
│   │   └── home.html
│   ├── pages
│   │   ├── index.html
│   │   └── user
│   │       └── {id}.html
│   └── public
│       ├── app.js
│       └── skin.css
├── go.mod
├── go.sum
└── main.go
```

> pages/user/{id}.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app">hello {{.Data.Name}}</div>
{{ end }}
```

> main.go
```go
	app.Get("/user/{id}", func(c *xun.Context) error {
		id := c.Request.PathValue("id")
		user := getUserById(id)
		return c.View(user)
	})
```

#### PageRoute with path parameters
A page file with a `{var}` segment auto-registers a PageRoute whose pattern matches Go 1.22's `http.ServeMux` syntax. To supply the model, override the same pattern with `app.Get`:

> pages/user/{id}.html
```html
<!--layout:home-->
{{ define "content" }}
    <h1>User #{{.Data.ID}}</h1>
    <p>{{.Data.Name}} ({{.Data.Email}})</p>
{{ end }}
```

> main.go
```go
app.Get("/user/{id}", func(c *xun.Context) error {
    id, _ := strconv.ParseInt(c.Request.PathValue("id"), 10, 64)
    var u User
    db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = ?", id).
        Scan(&u.ID, &u.Name, &u.Email)
    return c.View(u)        // ← .Data = u
})
```

> **Note:** `{id}` (Go 1.22 brace syntax) is the only form xun understands. The older `:id` colon-style does **not** work.

#### The `{$}` root pattern
`pages/index.html` auto-registers `GET /{$}`, not `GET /` (xun appends `{$}` to patterns that end in `/` so they only match with the trailing slash). To supply data to the root page you must register against the same pattern:

```go
// Right — matches the auto-registered PageRoute
app.Get("/{$}", handleLanding)

// Wrong — different pattern, will not be called for /
// app.Get("/", handleLanding)
```


### Multiple Viewers
In our application, a route can support multiple viewers. The response is rendered based on the `Accept` request header. If no viewer matches the `Accept` header, first registered viewer is used. For more examples, see the [Tests](app_test.go).

```bash
curl -v http://127.0.0.1
> GET / HTTP/1.1
> Host: 127.0.0.1
> User-Agent: curl/8.7.1
> Accept: */*
>
* Request completely sent off
< HTTP/1.1 200 OK
< Date: Thu, 26 Dec 2024 07:46:13 GMT
< Content-Length: 19
< Content-Type: text/plain; charset=utf-8
<
{"Name":"go-xun"}
```

> curl --header "Accept: text/html; \*/\*" http://127.0.0.1
```
> GET / HTTP/1.1
> Host: 127.0.0.1
> User-Agent: curl/8.7.1
> Accept: text/html; */*
>
* Request completely sent off
< HTTP/1.1 200 OK
< Date: Thu, 26 Dec 2024 07:49:47 GMT
< Content-Length: 343
< Content-Type: text/html; charset=utf-8
<
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Xun-Admin</title>
    <link rel="stylesheet" href="/skin.css">
<script type="text/javascript" src="/app.js"></script>
  </head>
  <body>

    <div id="app">hello go-xun</div>

  </body>
</html>
```
### Middleware
Middleware allows you to run code before a request is completed. Then, based on the incoming request, you can modify the response by rewriting, redirecting, modifying the request or response headers, or responding directly.

Integrating Middleware into your application can lead to significant improvements in performance, security, and user experience. Some common scenarios where Middleware is particularly effective include:

- Authentication and Authorization: Ensure user identity and check session cookies before granting access to specific pages or API routes.
- Server-Side Redirects: Redirect users at the server level based on certain conditions (e.g., locale, user role).
- Path Rewriting: Support A/B testing, feature rollout, or legacy paths by dynamically rewriting paths to API routes or pages based on request properties.
- Bot Detection: Protect your resources by detecting and blocking bot traffic.
- Logging and Analytics: Capture and analyze request data for insights before processing by the page or API.
- Feature Flagging: Enable or disable features dynamically for seamless feature rollout or testing.

> Authentication
```go
	admin := app.Group("/admin")

	admin.Use(func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			token := c.Request.Header.Get("X-Token")
			if !checkToken(token) {
				c.WriteStatus(http.StatusUnauthorized)
				return xun.ErrCancelled
			}
			return next(c)
		}
	})

```

> Logging
```go
	app.Use(func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			n := time.Now()
			defer func() {
				duration := time.Since(n)

				log.Println(c.Routing.Pattern, duration)
			}()
			return next(c)
		}
	})
```

### Multiple VirtualHosts
`net/http` package's router supports multiple host names that resolve to a single address by precedence rule.
For examples
```go
 mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {...})
 mux.HandleFunc("GET abc.com/", func(w http.ResponseWriter, req *http.Request) {...})
 mux.HandleFunc("GET 123.com/", func(w http.ResponseWriter, req *http.Request) {...})
```

In Page Router, we use `@` in top folder name to setup host rules in routing table. See more examples on [Tests](app_test.go)
```
├── app
│   ├── components
│   │   └── assets.html
│   ├── layouts
│   │   └── home.html
│   ├── pages
│   │   ├── @123.com
│   │   │   └── index.html
│   │   ├── index.html
│   │   └── user
│   │       └── {id}.html
│   └── public
│       ├── @abc.com
│       │   └── index.html
│       ├── app.js
│       └── skin.css
```

### Form and Validate
In an api application, we always need to collect data from request, and validate them. It is integrated with i18n feature as built-in feature now.

> check full examples on [Tests](./ext/form/binder_test.go)


```go
type Login struct {
		Email  string `form:"email" json:"email" validate:"required,email"`
		Passwd string `json:"passwd" validate:"required"`
	}
```

#### BindQuery
```go
	app.Get("/login", func(c *Context) error {
		it, err := form.BindQuery[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})
```

#### BindForm
```go
app.Post("/login", func(c *Context) error {
		it, err := form.BindForm[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})
```

#### BindJson
```go
app.Post("/login", func(c *Context) error {
		it, err := form.BindJson[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})
```

#### Validate Rules
Many [baked-in validations](https://github.com/go-playground/validator) are ready to use. Please feel free to check [docs](https://github.com/go-playground/validator?tab=readme-ov-file#usage-and-documentation) and write your custom validation methods.

#### i18n
English is default locale for all validate message. It is easy to add other locale.
```go
import(
  "github.com/go-playground/locales/zh"
  ut "github.com/go-playground/universal-translator"
  trans "github.com/go-playground/validator/v10/translations/zh"

)

xun.AddValidator(ut.New(zh.New()).GetFallback(), trans.RegisterDefaultTranslations)
```

> check more translations on [here](https://github.com/go-playground/validator/tree/master/translations)

### Templates: `.Data` vs `.TempData`
The template's `.` is bound to `ViewModel{TempData, Data}` — two distinct maps passed together to the engine. Pick the right one or you'll get a silent empty-value bug.

| | `.Data` | `.TempData` |
|---|---|---|
| **Holds** | The current page's main business object (a `User`, a `[]User`, anything passed as the first arg to `c.View`) | Cross-request auxiliary values (current `Session`, page `title`, error messages, anything set via `c.Set(k, v)`) |
| **Set in Go** | `c.View(data, name)` first arg | `c.Set(k, v)` and `c.Get(k)` (both operate on `TempData`) |
| **Accessed in templates** | `{{.Data.X}}` | `{{.TempData.X}}` |
| **Lifetime** | A single `c.View(...)` call | Whole request — middleware → handler → view all share the same map |

```go
// ── Business data via .Data ───────────────────────────
func handleUserList(c *xun.Context) error {
    var users []User
    // ... query ...
    return c.View(users)        // PageRoute handler; viewer name omitted
}

// Template:
{{ range .Data }}
  <tr><td>{{ .Name }}</td></tr>
{{ end }}

// ── Auxiliary data via .TempData ──────────────────────
func handleDashboard(c *xun.Context) error {
    c.Set("title", "Dashboard") // ← .TempData.title
    return c.View(nil)
}

// Layout:
<title>{{ .TempData.title }} - Xun Web</title>
```

### `c.View` — when to omit the viewer name
`c.View(data, name...)` picks the viewer from the route's viewer list, falling back to the first one. The auto-registered PageRoute already includes `HtmlViewer`, so most handlers can omit `name`:

| Handler call | When to use |
|---|---|
| `c.View(data)` | On a PageRoute (`pages/<X>.html` auto-registered the viewer). xun negotiates `HtmlViewer` vs `JsonViewer` from the `Accept` header. |
| `c.View(data, "index")` | On a manually-registered route (`app.Get` / `g.Get`) that needs to pick a specific page viewer. |
| `c.View(data, "views/users/list")` | For shared partials under `app/views/` that have no owning route. |

### `app.Mux()` — the raw-mux escape hatch
`app.Mux()` returns the underlying `*http.ServeMux`. Use it only when you need to hand a request to a third-party `http.Handler` that doesn't fit xun's `HandleFunc` shape — Prometheus exporter, pprof, OpenTelemetry collector, low-precedence 404 fallbacks, or any library that returns a concrete `http.Handler`.

The wrapped `ResponseWriter` already exposes `http.Flusher` and `http.Hijacker`, so WebSocket / SSE handlers do **not** need `app.Mux()`. Stay in `HandleFunc` and run the normal `app.Use` / `group.Use` middleware chain:

```go
// routes.go
app.Get("/api/ws", func(c *xun.Context) error {
    conn, buf, err := c.Response.Hijack()
    if err != nil { return err }
    defer conn.Close()
    // upgrade handshake + WebSocket framing on `conn` / `buf`...
})
```

Bypasses everything xun-managed — middleware, content negotiation, error-to-status mapping, access logs, even `X-Log-Id`. If the handler needs the same auth as the rest of the app, you must replicate it on the raw request. If you skip `WithMux(http.NewServeMux())` and fall back to `http.DefaultServeMux`, you pollute global state — always pass your own mux to `xun.New`.

### Extensions
#### GZip/Deflate handler
Set up the compression extension to interpret and respond to `Accept-Encoding` headers in client requests, supporting both GZip and Deflate compression methods.

```go
app := xun.New(WithCompressor(&GzipCompressor{}, &DeflateCompressor{}))
```

#### AutoTLS
Use `autotls.Configure` to set up servers for automatic obtaining and renewing of TLS certificates from Let's Encrypt.

```go
mux := http.NewServeMux()

app := xun.New(xun.WithMux(mux))

//...

httpServer := &http.Server{
	Addr: ":http",
	//...
}

httpsServer := &http.Server{
	Addr: ":https",
	//...
}

autotls.
	New(autotls.WithCache(autocert.DirCache("./certs")),
		autotls.WithHosts("abc.com", "123.com")).
	Configure(httpServer, httpsServer)

go httpServer.ListenAndServe()
go httpsServer.ListenAndServeTLS("", "")
```

#### Cookie
Cookies are a way to store information at the client end. see [more examples](./ext/cookie/cookie_test.go)
> Write cookie with base64(URL Encoding) to client
```go
cookie.Set(ctx,  http.Cookie{Name: "test", Value: "value"}) // Set-Cookie: test=dmFsdWU=
```

> Read and decoded cookie from client's request
```go
v, err := cookie.Get(ctx,"test")

fmt.Println(v) // value
```

When signed, the cookies can't be forged, because their values are validated using HMAC.
```go
ts, err := cookie.SetSigned(ctx,http.Cookie{Name: "test", Value: "value"},[]byte("secret")) // ts is current timestamp

v, ts, err := cookie.GetSigned(ctx, "test",[]byte("secret")) // v is value, ts is the timestamp that was signed
```

> Delete a cookie
```go
cookie.Delete(ctx, http.Cookie{Name: "test", Value: "dmFsdWU="}) // Set-Cookie: test=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0
```

#### HSTS
HTTP Strict Transport Security (HSTS) is a simple and widely supported standard to protect visitors by ensuring that their browsers always connect to a website over HTTPS.


> Redirect redirects plain HTTP requests to HTTPS. **DON'T use it if HTTPs is unsupported on your server.**
```go
app.Use(hsts.Redirect())
```

> Write HSTS header if it is a HTTPs request. **It is only applied in HTTPs request.**
```go
app.Use(hsts.WriteHeader())
```

#### Proxy Protocol
The PROXY protocol allows our application to receive client connection information that is passed through proxy servers and load balancers. Both PROXY protocol versions 1 and 2 are supported.

[How to use the Proxy Protocol to preserve a client's ip address?](https://www.haproxy.com/blog/use-the-proxy-protocol-to-preserve-a-clients-ip-address)

**Security Note: Do not enable the PROXY protocol on your servers unless they are located behind a proxy server or load balancer. If the PROXY protocol is enabled without such intermediaries, any client could potentially send fake IP addresses or other misleading information, posing a security risk.**

> ListenAndServe

```go
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":80",
		Handler: mux,
	}

	app := xun.New(WithMux(mux))
	app.Start()
	defer app.Close()

	//   srv.ListenAndServe()
	proxyproto.ListenAndServe(srv)
```

> ListenAndServeTLS

```go
	httpsServer := &http.Server{
		Addr:    ":443",
		Handler: mux,
	}

	autotls.New(autotls.WithCache(autocert.DirCache("./certs")),
		autotls.WithHosts("yaitoo.cn", "www.yaitoo.cn")).
		Configure(srv, httpsServer)

  // httpsServer.ListenAndServeTLS( "", "")
	proxyproto.ListenAndServeTLS(httpsServer, "", "")
```

#### Logging

Logs each incoming request to the provided logger. The format of the log messages is customizable using the `Format` option. The default format is the combined log format (XLF/ELF).

> Enable `reqlog` middleware

```go
func main(){
 	//....
  logger, _ := setupLogger()

  app.Use(reqlog.New(reqlog.WithLogger(logger),
		reqlog.WithUser(getUserID),
		reqlog.WithVisitor(getVisitorID),
		reqlog.WithFormat(reqlog.Combined))))
 	//...
}

func setupLogger() (*log.Logger, error) {
	logFile, err := os.OpenFile("./access.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return log.New(logFile, "", 0), nil
}

func getVisitorID(c *xun.Context) string {
	v, err := c.Request.Cookie("visitor_id") // use fingerprintjs to generate visitor id in client's cookie
	if err != nil {
		return ""
	}

	return v.Value
}

func getUserID(c *xun.Context) string {
	v, _, err := cookie.GetSigned(c, "session_id", secretKey)
	if err != nil {
		return ""
	}

	return v
}

```

> Install GoAccess to generate real-time analysis report

[How to install GoAccess](https://goaccess.io/get-started)

```bash
goaccess ./access.log --geoip-database=./GeoLite2-ASN.mmdb --geoip-database=./GeoLite2-City.mmdb -o ./realtime.html --log-format=COMBINED --real-time-html
```

> Serve the online real-time analysis report
```go
	app.Get("/reports/realtime.html", func(c *xun.Context) error {
		http.ServeFile(c.Response, c.Request, "./realtime.html")
		return nil
	})
```

#### CSRF Token
A CSRF (Cross-Site Request Forgery) token is a unique security measure designed to protect web applications from unauthorized or malicious requests. see more [examples](./ext/csrf/csrf_test.go)

> Enable `csrf` middleware
```go
func main(){
 	//....
  secretKey := []byte("your-secret-key")

  app.Use(csrf.New(secretKey))
 	//...
}
```

> Enable `JsToken` to prevent bot requests on POST/PUT/DELETE

- enable `csrf` with JsToken
```go
func main(){
 	//....
  secretKey := []byte("your-secret-key")
  app.Use(csrf.New(secretKey,csrf.WithJsToken()))
  // ...
  app.Get("/assets/csrf.js",csrf.HandleFunc(secretKey))
  //...
}
```

- load `csrf.js` on html
```html
<script type="text/javascript" src="/assets/csrf.js" defer></script>
```


#### Access Control List ([ACL](./ext/acl/))
The ACL filters and monitors HTTP traffic through granular rule sets, designed to protect web applications/APIs from malicious bots, exploit attempts, and unauthorized access.

##### Core Filtering Dimensions
- Host-Based Filtering (AllowHosts)

    Restrict access to explicitly permitted domains/subdomains
- IP Range Control (AllowIPNets/DenyIPNets)

    Allow/block traffic from specific IP addresses or CIDR-notated subnets. IPv4/IPv6 are both supported.
- Geolocation Filtering (AllowCountries/DenyCountries)

    Permit/restrict access based on client geolocation

##### Enforcement Actions
- Block unauthorized requests with 403 Forbidden status
- Host Redirection (Conditional):

    When AllowHosts validation fails:
    - Redirect to HostRedirectURL
    - Use customizable HTTP status HostRedirectStatusCode (e.g., 307 Temporary Redirect)

##### Code Examples
see more [examples](./ext/acl/acl_test.go)

> AllowHosts
```go
app.Use(acl.New(acl.AllowHosts("abc.com","123.com"), acl.WithHostRedirect("https://abc.com", 302)))

```

> Whitelist Mode by IPNets
```go
app.Use(acl.New(acl.AllowIPNets("172.0.0.1","2000::1/8")),acl.DenyIPNets("*"))
```

> Whitelist Mode by Countries
```go
func lookup(ip string)string {
	db, _ := geoip2.Open("./GeoLite2-City.mmdb")
	nip := net.ParseIP(ip)

	c, _ := db.cityDB.City(nip)

	return c.CountryCode
}

app.Use(acl.New(acl.WithLookupFunc(lookup),
	acl.AllowCountries("CN"),acl.DenyCountries("*")))
```

> Blacklist Mode by IPNets
```go
app.Use(acl.DenyIPNets("172.0.0.0/24"))
```

> Blacklist Mode by Countries
```go
app.Use(acl.New(acl.WithLookupFunc(lookup),acl.DenyCountries("us","cn")))
```

##### Config Example
The optimal solution is to load the rules from a configuration file rather than hard-coding them. The ACL system also monitors the configuration file for changes and automatically reloads the rules. see more [examples](./ext/acl/config_test.go)

> config file
```ini
[allow_hosts]
abc.com
www.abc.com
[allow_ipnets]
89.207.132.170/24
# ::1
; 127.0.0.1
[deny_ipnets]
*
[allow_countries]

[deny_countries]
us

[host_redirect]
url=http://yaitoo.cn
status_code=302

```

> use middleware with config
```go
app.Use(acl.New(acl.WithConfig("./acl.ini")))
```

#### Server-Sent Events ([SSE](./ext/sse/))
Server-Sent Events (SSE) is a server push technology enabling a client to receive automatic updates from a server via an HTTP connection.

> use `sse` extension to handle SSE request
```go
ss := sse.New()

app.Get("/topic/{id}", func(ctx *xun.Context)error {
	id := c.Get("SessionID").(string)
	s, err := ss.Join(c.Request.Context(), id, c.Response)
	if err != nil {
		c.WriteStatus(http.StatusBadRequest)
		return xun.ErrCancelled
	}

	s.Wait()

	ss.Leave(id)

	return nil
})

```

> push an event to the user
```go
u := ss.Get("user_id")
if u != nil {
	u.Send(sse.TextEvent{
		Name:"showMessage",
		Data:"Hello",
	})
}
```

> broadcast an event to all users
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
ss.Broadcast(ctx, sse.TextEvent{
	Name:"shutdown",
	Data:"Server is shutting down",
}
```

> shutdown server and close all user connections
```go
ss.Shutdown()
```

> use [htmx-ext-sse](https://htmx.org/extensions/sse/) extension to send SSE request
```html

<script src="https://cdnjs.cloudflare.com/ajax/libs/htmx/2.0.4/ext/sse.min.js" integrity="sha512-uROW42fbC8XT6OsVXUC00tuak//shtU8zZE9BwxkT2kOxnZux0Ws8kypRr2UV4OhTEVmUSPIoUOrBN5DXeRNAQ=="
crossorigin="anonymous" referrerpolicy="no-referrer"></script>

<div class="w-full" hx-ext="sse" sse-connect="/topic/{id}" >
...
</div>
```

### Works with [tailwindcss](https://tailwindcss.com/docs/installation)
#### 1. Install Tailwind CSS
Install tailwindcss via npm, and create your tailwind.config.js file.
```bash
npm install -D tailwindcss
npx tailwindcss init
```
#### 2. Configure your template paths
Add the paths to all of your template files in your tailwind.config.js file.

> tailwind.config.js
```json
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./app/**/*.{html,js}"],
  theme: {
    extend: {},
  },
  plugins: [],
}
```

#### 3. Add the Tailwind directives to your CSS
Add the @tailwind directives for each of Tailwind’s layers to your main CSS file.
> app/tailwind.css
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

#### 4. Start the Tailwind CLI build process
Run the CLI tool to scan your template files for classes and build your CSS.

```bash
npx tailwindcss -i ./app/tailwind.css -o ./app/public/theme.css --watch
```

#### 5. Start using Tailwind in your HTML
Add your compiled CSS file to the `assets.html` and start using Tailwind’s utility classes to style your content.

> components/assets.html
```html
<link rel="stylesheet" href="/skin.css">
<link rel="stylesheet" href="/theme.css">
<script type="text/javascript" src="/app.js"></script>
```

### Works with [htmx.js](https://htmx.org/docs/)
#### 1. Add new pages
> `pages/admin/index.html` and `pages/login.html`
```
├── app
│   ├── components
│   │   └── assets.html
│   ├── layouts
│   │   └── home.html
│   ├── pages
│   │   ├── @123.com
│   │   │   └── index.html
│   │   ├── admin
│   │   │   └── index.html
│   │   ├── index.html
│   │   ├── login.html
│   │   └── user
│   │       └── {id}.html
│   ├── public
│   │   ├── @abc.com
│   │   │   └── index.html
│   │   ├── app.js
│   │   ├── skin.css
│   │   └── theme.css
│   ├── tailwind.css
```

#### 2. Serve [htmx-ext.js](./ext/htmx/htmx.js) library
The library to enable seamless integration between native JavaScript methods and htmx features, enhancing interactive capabilities without compromising core functionality.

```go
	app.Get("/htmx-ext.js", htmx.HandleFunc())
```

#### 3. Install htmx.js and htmx-ext.js

> components/assets.html
```html
<link rel="stylesheet" href="/skin.css">
<link rel="stylesheet" href="/theme.css">
<script src="https://unpkg.com/htmx.org@2.0.4" integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+" crossorigin="anonymous"></script>
<script type="text/javascript" src="/htmx-ext.js"></script>
<script type="text/javascript" src="/app.js" defer></script>
```

#### 4. Enabled `htmx` feature on pages
> pages/index.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app" class="text-3xl font-bold underline" hx-boost="true">

			{{ if .TempData.Session }}
				Hello {{ .TempData.Session }}, go <a href="/admin">Admin</>
			{{ else }}
        Hello guest, please <a href="/login">Login</a>
			{{ end }}
    </div>

{{ end }}
```

> pages/login.html
```html
<!--layout:home-->
{{ define "content" }}

<div class="flex min-h-full flex-col justify-center px-6 py-12 lg:px-8">
  <div class="sm:mx-auto sm:w-full sm:max-w-sm">
    <h2 class="mt-10 text-center text-2xl/9 font-bold tracking-tight text-gray-900">Sign in to your account</h2>
  </div>

  <div class="mt-10 sm:mx-auto sm:w-full sm:max-w-sm">
    <form class="space-y-6" action="#" method="POST" hx-post="/login">
      <div>
        <label for="email" class="block text-sm/6 font-medium text-gray-900">Email address</label>
        <div class="mt-2">
          <input type="email" name="email" id="email" autocomplete="email" required class="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6">
        </div>
      </div>

      <div>
        <div class="flex items-center justify-between">
          <label for="password" class="block text-sm/6 font-medium text-gray-900">Password</label>
        </div>
        <div class="mt-2">
          <input type="password" name="password" id="password" autocomplete="current-password" required class="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6">
        </div>
      </div>

      <div>
        <button type="submit" class="flex w-full justify-center rounded-md bg-indigo-600 px-3 py-1.5 text-sm/6 font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">Sign in</button>
      </div>
    </form>
  </div>
</div>

{{ end }}
```

> pages/admin/index.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app" class="text-3xl font-bold underline">
				Hello admin: {{ .Data.Name }}
			</div>
{{ end }}
```

#### 5. Setup Hx-Trigger listener
> app.js
```js
$x.ready(function(evt) {
	document.addEventListener("showMessage", function(evt){
    alert(evt.detail);
  })
},'body');

```

#### 6. Apply `htmx` interceptor
```go

	app := xun.New(xun.WithInterceptor(htmx.New()))

```

#### 7. Create router handler to process request
create an `admin` group router, and apply a middleware to check if it's logged. if not, redirect to /login.


```go
	admin := app.Group("/admin")

	admin.Use(func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			s, err := c.Request.Cookie("session")
			if err != nil || s == nil || s.Value == "" {
				c.Redirect("/login?return=" + c.Request.URL.String())
				return xun.ErrCancelled
			}

			// set session in Context.TempData,
			// and get it by `.TempData.Session on text/html template files
			c.Set("Session", s.Value)
			return next(c)
		}
	})

	admin.Get("/{$}", func(c *xun.Context) error {
		return c.View(User{
			Name: c.Get("session").(string),
		})
	})

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

		if it.Data.Email != "xun@yaitoo.cn" || it.Data.Password != "123" {
			htmx.WriteHeader(c,htmx.HxTrigger, htmx.HxHeader[string]{
				"showMessage": "Email or password is incorrect",
			})
			c.WriteStatus(http.StatusBadRequest)
			return c.View(it)
		}

		cookie := http.Cookie{
			Name:     "session",
			Value:    it.Data.Email,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		}

		http.SetCookie(c.Response, &cookie)

    u, _ := url.Parse(c.RequestReferer())

		c.Redirect(u.Query().Get("return"))
		return nil
	})
```

## Deploy your application
Leveraging Go's built-in `//go:embed` directive and the standard library's `fs.FS` interface, we can compile all static assets and configuration files into a single self-contained binary. This dependency-free approach enables seamless deployment to any server environment.

```go

//go:embed app
var fsys embed.FS

func main() {
	var dev bool
	flag.BoolVar(&dev, "dev", false, "it is development environment")

	flag.Parse()

	var opts []xun.Option
	if dev {
		// use local filesystem in development, and watch files to reload automatically
		opts = []xun.Option{xun.WithFsys(os.DirFS("./app")), xun.WithWatch()}
	} else {
		// use embed resources in production environment
		views, _ := fs.Sub(fsys, "app")
		opts = []xun.Option{xun.WithFsys(views)}
	}

	app := xun.New(opts...)
	//...

	app.Start()
	defer app.Close()

	if dev {
		slog.Default().Info("xun-admin is running in development")
	} else {
		slog.Default().Info("xun-admin is running in production")
	}

	err := http.ListenAndServe(":80", http.DefaultServeMux)
	if err != nil {
		panic(err)
	}
}
```

## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to Xun](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)
