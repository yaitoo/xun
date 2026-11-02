package xun

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// sitemapMaxURLs is the protocol limit for a single sitemap file: a sitemap
// must not contain more than 50,000 URLs
// (https://www.sitemaps.org/protocol.html).
const sitemapMaxURLs = 50000

// sitemapEffectiveURLs is the effective cap the framework enforces on every
// document it serves: 40,000 URLs, 80% of the protocol limit.
//
// A crawler rejects an oversized document as a whole, so serving one loses
// every URL in it — not just the ones past the limit. The framework
// therefore truncates at the effective cap and logs the drop, which keeps
// the first 40,000 URLs indexable.
//
// The headroom exists to stay clear of the protocol limit, not to keep
// serving close to it: the warning and the truncation trigger at the same
// threshold.
const sitemapEffectiveURLs = sitemapMaxURLs * 4 / 5

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
// The served document is capped: the framework renders at most 40,000 URLs
// (80% of the protocol's 50,000 URL limit) and logs the entries it drops.
// A crawler rejects an oversized document as a whole, so every URL in it
// would be lost — truncating keeps the first ones indexable. To publish
// more than 40,000 URLs, split them across several groups and register
// App.SitemapIndex, which renders one child sitemap file per group.
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
//
// A nil builder panics at registration: it is a programming error, and
// failing here keeps the stack trace pointing at the call site instead of
// at a request goroutine.
func (app *App) Sitemap(builders ...SitemapBuilder) {
	for i, b := range builders {
		if b == nil {
			// Fail at registration: a nil builder would otherwise panic
			// inside the request goroutine, where the stack trace points
			// at the framework instead of the call site.
			panic(fmt.Sprintf("xun: Sitemap: nil builder at index %d", i))
		}
	}

	if len(builders) == 0 {
		builders = []SitemapBuilder{defaultSitemapBuilder}
	}

	if _, ok := app.routes[sitemapKey]; ok && !app.sitemapRouteOwned {
		app.logger.Warn("xun: sitemap replaces the route " + sitemapKey +
			" registered outside the sitemap machinery")
	}

	// WithViewer() with no arguments clears the app's default handler
	// viewers: this route renders the named "sitemap.xml" viewer, it does
	// not negotiate one.
	app.createHandler(sitemapKey, sitemapHandler(builders), []RoutingOption{WithViewer()}, app)

	// The route is ours now; a later app.Get("/sitemap.xml", ...) hands
	// ownership back to the user (see App.createHandler).
	app.sitemapRouteOwned = true
}

// sitemapHandler returns the handler serving GET /sitemap.xml.
//
// The viewer is resolved from app.viewers on every request — not once at
// registration time — so a template published (or hot-reloaded) after the
// route was registered is picked up without re-registering the route. When
// no viewer is published the route answers 404.
//
// The entries are collected before the viewer is looked up: a builder that
// fails is a server-side failure (500 + X-Log-Id) even when no template is
// published yet, so the error is not masked by the 404.
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

// muxHasPattern reports whether the mux already serves the given method and
// path with a pattern of its own. Registering such a pattern again panics
// with ServeMux's duplicate-pattern panic, so the framework checks before it
// registers the sitemap route.
//
// A catch-all pattern ("GET /") does not count: the framework's more
// specific pattern can live next to it.
func muxHasPattern(mux *http.ServeMux, method, path string) bool {
	req := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
		Host:   "localhost",
	}

	_, pattern := mux.Handler(req)
	if pattern == "" {
		return false
	}

	// A pattern reads "METHOD PATH", "HOST PATH" or "PATH"; only the path
	// decides whether it collides with the framework's route.
	if i := strings.IndexAny(pattern, " \t"); i >= 0 {
		pattern = pattern[i+1:]
	}

	return pattern == path
}

// collectSitemapURLs runs the builders in order and merges their entries.
//
// The entries are concatenated in builder order and de-duplicated by the
// exact Loc string, keeping the first occurrence. The result is not
// re-sorted: a builder's ordering is a contract — the default builder's
// order is the sorted order of App.SitemapURLs.
//
// The result is truncated to the effective cap (see truncateSitemapURLs):
// the framework never serves a document a crawler would reject as a whole.
func collectSitemapURLs(c *Context, builders []SitemapBuilder) ([]SitemapURL, error) {
	var out []SitemapURL
	seen := make(map[string]struct{})

	for _, b := range builders {
		urls, err := b(c)
		if err != nil {
			return nil, err
		}

		out = dedupSitemapURLs(out, seen, urls)
	}

	return truncateSitemapURLs(c, "sitemap", out), nil
}

// dedupSitemapURLs appends the entries whose Loc was not seen yet to out and
// returns the result. The first occurrence of a Loc wins; the later ones are
// dropped.
//
// The seen map is owned by the caller, so the same Loc can be de-duplicated
// across several builders (the single-document mode) or per group (the index
// mode).
func dedupSitemapURLs(out []SitemapURL, seen map[string]struct{}, urls []SitemapURL) []SitemapURL {
	for _, u := range urls {
		if _, ok := seen[u.Loc]; ok {
			continue
		}
		seen[u.Loc] = struct{}{}
		out = append(out, u)
	}

	return out
}

// truncateSitemapURLs cuts urls down to the effective cap.
//
// A crawler rejects an oversized document as a whole, so every URL in it is
// lost — serving one is worse than serving a truncated document. The
// framework therefore keeps the first sitemapEffectiveURLs URLs and logs
// the total, the cap and the number of dropped entries.
//
// scope names the document the entries belong to ("sitemap", "sitemap
// index", "sitemap group <name>").
func truncateSitemapURLs(c *Context, scope string, urls []SitemapURL) []SitemapURL {
	if len(urls) <= sitemapEffectiveURLs {
		return urls
	}

	total := len(urls)
	dropped := total - sitemapEffectiveURLs

	c.App.logger.Warn("xun: sitemap exceeds the "+strconv.Itoa(sitemapEffectiveURLs)+" URL limit: "+
		strconv.Itoa(total)+" URLs collected, "+strconv.Itoa(dropped)+" dropped",
		slog.String("scope", scope),
		slog.Int("urls", total),
		slog.Int("limit", sitemapEffectiveURLs),
		slog.Int("dropped", dropped))

	return urls[:sitemapEffectiveURLs]
}

// defaultSitemapBuilder returns the content engine's entries — the same
// list App.SitemapURLs returns.
func defaultSitemapBuilder(c *Context) ([]SitemapURL, error) {
	return c.App.SitemapURLs(), nil
}
