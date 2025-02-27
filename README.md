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



## Building your application
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


#### Access Control List
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
The optimal solution is to load the rules from a configuration file rather than hard-coding them. The ACL system also monitors the configuration file for changes and automatically reloads the rules. see more [examples](./ext/acl//config_test.go) 

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

app.Get("/sse/{id}", func(ctx *xun.Context)error {
	client, err := ss.Join(c.Request.Context(), c.Request.PathValue("id"), c.Response)
	if err != nil {
		c.WriteStatus(http.StatusBadRequest)
		return xun.ErrCancelled
	}

	client.Wait()

	ss.Leave(id.Value)

	return nil
})

```

> push an Event to client	
```go
client := ss.Get("id")
if client != nil {
	client.Send(sse.Event{
		Name:"showMessage",
		Data:"Hello World",
	})
}
```

> broadcast an Event to all clients
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
ss.Broadcast(ctx, sse.Event{
	Name:"shutdown",
	Data:"Server is shutting down",
}
```

> use [htmx-ext-sse](https://htmx.org/extensions/sse/) extension to send SSE request
```html

<script src="https://cdnjs.cloudflare.com/ajax/libs/htmx/2.0.4/ext/sse.min.js" integrity="sha512-uROW42fbC8XT6OsVXUC00tuak//shtU8zZE9BwxkT2kOxnZux0Ws8kypRr2UV4OhTEVmUSPIoUOrBN5DXeRNAQ==" 
crossorigin="anonymous" referrerpolicy="no-referrer"></script>

<div class="w-full" hx-ext="sse" sse-connect="/chat/{id}" >
...
</div>
```

### Deploy your application
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


## Contributing
Contributions are welcome! If you're interested in contributing, please feel free to [contribute to Xun](CONTRIBUTING.md)


## License
[Apache-2.0 license](LICENSE)