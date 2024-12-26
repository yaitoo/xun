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
go-htmx has some specified folders that is used to organize code, routing and static assets.
- `public`: Static assets to be served. 
- `components` A partial view that is shared between layouts/pages/views.
- `views`: A internal page view. It is used in `context.View` to render different view from current routing.
- `layouts`: A layout is shared between multiple pages/views
- `pages`: A public page view. It also is used to automatically create a accessible page routing.


### Layouts and Pages
go-htmx uses file-system based routing, meaning you can use folders and files to define routes. This page will guide you through how to create layouts and pages, and link between them.

#### Creating a page
A page is UI that is rendered on a specific route. To create a page, add a page file(.html) inside the pages directory. For example, to create an index page (/):
```
└── app
    └── pages
        └── index.html
```

``` html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Htmx-Admin</title>
  </head>
  <body>
    <div id="app"></div>
  </body>
</html>
```

## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to go-htmx](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)