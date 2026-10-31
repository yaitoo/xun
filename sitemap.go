package xun

import (
	"net/http"
)

// Sitemap convention: a public/sitemap.xml file is parsed as a text/template
// (via TextTemplate) and rendered through a TextViewer. The viewer is always
// exposed as app.viewers["sitemap.xml"] so user handlers can call
// c.View(data, "sitemap.xml") to reuse it — the viewer name matches the
// route URL the sitemap is served from.
//
// The route itself belongs to the framework (see App.Sitemap), not to the
// static view engine: the engine only publishes the viewer.
const (
	sitemapPath = "public/sitemap.xml" // fsys path; kept verbatim for fs.ReadFile/Stat
	sitemapKey  = "GET /sitemap.xml"   // routes-map key
	sitemapName = "sitemap.xml"        // app.viewers key; matches the route path
)

// SitemapBuilder assembles the entries rendered into GET /sitemap.xml.
//
// The builder receives the request Context so it can read request-scoped
// state and use c.Request.Context() for database or HTTP calls. Returning
// an error aborts the request with the framework's standard error mapping
// (500 + X-Log-Id): a sitemap that silently misses half of its URLs is
// worse than an error a crawler can retry.
//
// Entries are used verbatim — the framework neither resolves a host nor
// normalizes a path. The template (or the builder) owns the scheme and host.
type SitemapBuilder func(c *Context) ([]SitemapURL, error)

// Sitemap registers the GET /sitemap.xml route and renders the viewer named
// "sitemap.xml" (published by the static view engine when public/sitemap.xml
// exists).
//
// With no builder the content engine's entries are used (the same list
// App.SitemapURLs returns). With several builders their entries are
// concatenated in builder order, de-duplicated by Loc (first occurrence
// wins), and not re-sorted.
//
//	app.Sitemap(func(c *xun.Context) ([]xun.SitemapURL, error) {
//		rows, err := db.QueryContext(c.Request.Context(), "SELECT slug FROM posts")
//		if err != nil {
//			return nil, err
//		}
//		urls := make([]xun.SitemapURL, 0, len(rows))
//		for _, r := range rows {
//			urls = append(urls, xun.SitemapURL{Loc: "/blog/" + r.Slug})
//		}
//		return urls, nil
//	})
//
// Registration order decides precedence: calling Sitemap again replaces the
// previous registration, and a route registered with app.Get afterwards
// replaces it in turn. Replacing a route the framework does not own logs a
// warning.
//
// The route answers 404 until a viewer named "sitemap.xml" is published —
// app.Sitemap does not require public/sitemap.xml to exist.
func (app *App) Sitemap(builders ...SitemapBuilder) {
	if len(builders) == 0 {
		builders = []SitemapBuilder{defaultSitemapBuilder}
	}

	if _, ok := app.routes[sitemapKey]; ok && !app.sitemapRouteOwned {
		app.logger.Warn("xun: sitemap replaces the route " + sitemapKey +
			" registered outside the sitemap machinery")
	}

	app.createHandler(sitemapKey, sitemapHandler(builders), []RoutingOption{WithViewer()}, app)

	// The route is ours now; a later app.Get("/sitemap.xml", ...) hands
	// ownership back to the user (see App.HandleFunc).
	app.sitemapRouteOwned = true
}

// sitemapHandler returns the handler serving GET /sitemap.xml.
//
// The viewer is resolved from app.viewers on every request — not once at
// registration time — so a template published (or hot-reloaded) after the
// route was registered is picked up without re-registering the route. When
// no viewer is published the route answers 404.
//
// The named viewer is rendered directly rather than through c.View: c.View
// falls back to Accept-header negotiation over the route's viewers, which
// would answer a JSON request with JSON instead of the sitemap.
func sitemapHandler(builders []SitemapBuilder) HandleFunc {
	return func(c *Context) error {
		entries, err := collectSitemapURLs(c, builders)
		if err != nil {
			return err
		}

		v, ok := c.App.viewers[sitemapName]
		if !ok {
			c.WriteStatus(http.StatusNotFound)
			return nil
		}

		return v.Render(c, entries)
	}
}

// collectSitemapURLs runs the builders in order and merges their entries.
//
// The entries are concatenated in builder order and de-duplicated by the
// exact Loc string, keeping the first occurrence. The result is not
// re-sorted: a builder's ordering is a contract — the default builder's
// order is the sorted order of App.SitemapURLs.
func collectSitemapURLs(c *Context, builders []SitemapBuilder) ([]SitemapURL, error) {
	var out []SitemapURL
	seen := make(map[string]struct{})

	for _, b := range builders {
		urls, err := b(c)
		if err != nil {
			return nil, err
		}

		for _, u := range urls {
			if _, ok := seen[u.Loc]; ok {
				continue
			}
			seen[u.Loc] = struct{}{}
			out = append(out, u)
		}
	}

	return out, nil
}

// defaultSitemapBuilder returns the content engine's entries — the same
// list App.SitemapURLs returns.
func defaultSitemapBuilder(c *Context) ([]SitemapURL, error) {
	return c.App.SitemapURLs(), nil
}
