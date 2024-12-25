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
  

## Tutorials

### Install go-htmx
- install latest commit from `main` branch
```
go get github.com/yaitoo/htmx@main
```

- install latest release
```
go get github.com/yaitoo/htmx@latest
```

### Create a website by StaticViewEngine

### Create a website by HtmlViewEngine

### Create an api application by JsonViewEngine

### Create a hybrid web application  

## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to go-htmx](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)