# Go-Htmx
go-htmx is a HTTP web framework based on Go's built-in `html/template` and `http/ServeMux`.

[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)
[![Tests](https://github.com/yaitoo/htmx/actions/workflows/tests.yml/badge.svg)](https://github.com/yaitoo/htmx/actions/workflows/tests.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/yaitoo/htmx.svg)](https://pkg.go.dev/github.com/yaitoo/htmx)
[![Codecov](https://codecov.io/gh/yaitoo/htmx/branch/main/graph/badge.svg)](https://codecov.io/gh/yaitoo/htmx)
[![GitHub Release](https://img.shields.io/github/v/release/yaitoo/htmx)](https://github.com/yaitoo/htmx/blob/main/CHANGELOG.md)
[![Go Report Card](https://goreportcard.com/badge/yaitoo/htmx)](http://goreportcard.com/report/yaitoo/htmx)

## Features
- Works with Go's built-in `ServeMux` router that was introduced in 1.22. [Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements).
- Works with Go's built-in `html/template`. It is built-in support for Server-Side Rendering (SSR).
- Support mixed view engine: `StaticViewEngine`, `JsonViewEngine` and `HtmlViewEngine`.
- Support to automatically reload changed files in development environment.
  

## Getting Started

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

*NB: All html files(component,layout, view and page) will be parsed by [html/template](https://pkg.go.dev/html/template). You can feel free to use any built-in feature in the official [features](https://pkg.go.dev/text/template), and your custom functions that is registered in `HtmlViewEngine`.*

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

### Creating a layout
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
    {{ template "content" .}}
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
### Creating a assets component
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
   {{ template "components/assets" . }}
  </head>
  <body>
    {{ template "content" .}}
  </body>
</html>
```

## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to go-htmx](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)