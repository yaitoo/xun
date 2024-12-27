# GO-HTMX
go-htmx is a HTTP web framework based on Go's built-in html/template and net/http package’s router.

[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)
[![Tests](https://github.com/yaitoo/htmx/actions/workflows/tests.yml/badge.svg)](https://github.com/yaitoo/htmx/actions/workflows/tests.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/yaitoo/htmx.svg)](https://pkg.go.dev/github.com/yaitoo/htmx)
[![Codecov](https://codecov.io/gh/yaitoo/htmx/branch/main/graph/badge.svg)](https://codecov.io/gh/yaitoo/htmx)
[![GitHub Release](https://img.shields.io/github/v/release/yaitoo/htmx)](https://github.com/yaitoo/htmx/blob/main/CHANGELOG.md)
[![Go Report Card](https://goreportcard.com/badge/yaitoo/htmx)](http://goreportcard.com/report/yaitoo/htmx)

## Features
- Works with Go's built-in `net/http.ServeMux` router. that was introduced in 1.22. [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements).
- Works with Go's built-in `html/template`. It is built-in support for Server-Side Rendering (SSR).
- Built-in Form and Validate feature with i18n support.
- Support mixed viewer by ViewEngines: `StaticViewEngine`, `JsonViewEngine` and `HtmlViewEngine`. You can feel free to add custom view engine, eg `XmlViewEngine`.
- Support to automatically reload changed files in development environment.
  

## Getting Started
> See full source code on [htmx-examples](https://github.com/yaitoo/htmx-examples)

### Install go-htmx
- install latest commit from `main` branch
```
go get github.com/yaitoo/htmx@main
```

- install latest release
```
go get github.com/yaitoo/htmx@latest
```

### Project structure
go-htmx has some specified directories that is used to organize code, routing and static assets.
- `public`: Static assets to be served. 
- `components` A partial view that is shared between layouts/pages/views.
- `views`: A internal page view. It is used in `context.View` to render different view from current routing.
- `layouts`: A layout is shared between multiple pages/views
- `pages`: A public page view. It also is used to automatically create a accessible page routing.

*NB: All html files(component,layout, view and page) will be parsed by [html/template](https://pkg.go.dev/html/template). You can feel free to use all built-in [Actions,Pipelines and Functions](https://pkg.go.dev/text/template), and your custom functions that is registered in `HtmlViewEngine`.*

### Layouts and Pages
go-htmx uses file-system based routing, meaning you can use folders and files to define routes. This section will guide you through how to create layouts and pages, and link between them.


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
    <title>Htmx-Admin</title>
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
    <title>Htmx-Admin</title>
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

*NB: `public/index.html` will be visited by `/` instead of `/index.html`.*

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
        └── skin.js
```      
> components/assets.html
```html
<link rel="stylesheet" href="/theme.css">
<script type="text/javascript" src="/app.js"></script>
```
> layouts/home.html
```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Htmx-Admin</title>
    {{ block "components/assets" . }} {{ end }}
  </head>
  <body>
    {{ block "content" .}} {{ end }}
  </body>
</html>
```

## Building your application
### Routing
#### Route Handler
Page Router only serve static content from html files. We have to define router handler in go to process request and bind data to the template file via `HtmlViewer`. 

> pages/index.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app">hello {{.Name}}</div>
{{ end }}
```

> main.go
```go
	app.Get("/{$}", func(c *htmx.Context) error {
		return c.View(map[string]string{
			"Name": "go-htmx",
		})
	})
```


*NB: An `/index.html` always be registered as `/{$}` in routing table. See more detail on [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements).*
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
│       └── skin.js
├── go.mod
├── go.sum
└── main.go
```

> pages/user/{id}.html
```html
<!--layout:home-->
{{ define "content" }}
    <div id="app">hello {{.Name}}</div>
{{ end }}
```

> main.go
```go
	app.Get("/user/{id}", func(c *htmx.Context) error {
		id := c.Request().PathValue("id")
		user := getUserById(id)
		return c.View(user)
	})
```


### Mixed Viewer
In our application, a routing can have multiple viewers. Response is render based on the request header `Accept`. Default viewer is used if there is no any viewer is matched by `Accept`. The built-it default viewer is `JsonViewer`. But it can be overridden by `htmx.WithViewer` in `htmx.New`. see more examples on [Tests](app_test.go)

> curl -v http://127.0.0.1
```
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
{"Name":"go-htmx"}
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
    <title>Htmx-Admin</title>
    <link rel="stylesheet" href="/theme.css">
<script type="text/javascript" src="/app.js"></script>
  </head>
  <body>

    <div id="app">hello go-htmx</div>

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

	admin.Use(func(next htmx.HandleFunc) htmx.HandleFunc {
		return func(c *htmx.Context) error {
			token := c.Request().Header.Get("X-Token")
			if !checkToken(token) {
				c.WriteStatus(http.StatusUnauthorized)
				return htmx.ErrCancelled
			}
			return next(c)
		}
	})

```

> Logging
```go
	app.Use(func(next htmx.HandleFunc) htmx.HandleFunc {
		return func(c *htmx.Context) error {
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
│       └── skin.js
```

### Form and Validate
In an api application, we always need to collect data from request, and validate them. It is integrated with i18n feature as built-in feature now.

> check full examples on [Tests](binder_test.go)


```go
type Login struct {
		Email  string `form:"email" json:"email" validate:"required,email"`
		Passwd string `json:"passwd" validate:"required"`
	}
```

#### BindQuery
```go
	app.Get("/login", func(c *Context) error {
		it, err := BindQuery[Login](c.Request())
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "htmx@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})
```

#### BindForm
```go
app.Post("/login", func(c *Context) error {
		it, err := BindForm[Login](c.Request())
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "htmx@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		return ErrCancelled
	})
```

#### BindJson
```go
app.Post("/login", func(c *Context) error {
		it, err := BindJson[Login](c.Request())
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "htmx@yaitoo.cn" && it.Data.Passwd == "123" {
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

htmx.AddValidator(ut.New(zh.New()).GetFallback(), trans.RegisterDefaultTranslations)
```

> check more translations on [here](https://github.com/go-playground/validator/tree/master/translations)

### Works with tailwindcss

### Works with htmx

## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to go-htmx](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)