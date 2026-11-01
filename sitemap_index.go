package xun

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Sitemap index convention: a public/sitemap-index.xml file is parsed as a
// text/template (via TextTemplate) and rendered through a TextViewer, the
// same way public/sitemap.xml is. The viewer is exposed as
// app.viewers["sitemap-index.xml"] so user handlers can call
// c.View(data, "sitemap-index.xml") to reuse it.
//
// The route itself belongs to the framework (see App.SitemapIndex), not to
// the static view engine: the engine only publishes the viewer.
const (
	sitemapIndexPath = "public/sitemap-index.xml" // fsys path; kept verbatim for fs.ReadFile/Stat
	sitemapIndexName = "sitemap-index.xml"        // app.viewers key; matches the template name

	// sitemapChildPattern is the route serving the child sitemap files.
	// Go 1.22's http.ServeMux requires a wildcard to be a complete path
	// segment, so the escaped group name travels as the wildcard's value:
	// "/sitemaps/shoes.xml" and "/sitemaps/%E5%A5%B3%E9%9E%8B" both match.
	sitemapChildPattern = "GET /sitemaps/{name}"

	// sitemapChildPrefix is the URL prefix of a child sitemap file. The
	// group name follows it, URL-escaped.
	sitemapChildPrefix = "/sitemaps/"
)

// SitemapGroup is one named collection of URLs rendered as one child sitemap
// file inside a sitemap index.
//
// The sitemap protocol caps a document at 50,000 URLs; anything larger is
// published as a sitemap index — a <sitemapindex> document listing child
// sitemap files, each of which stays within the limit. A group is one of
// those child files.
//
//   - Name — the child file's name, e.g. "shoes.xml". It becomes the file's
//     path segment: the child file is served at "/sitemaps/" plus the
//     URL-escaped name. The framework appends no extension and no sequence
//     number, so the name is the file name the application chose.
//
//   - URLs — the entries of the child file. They are rendered by the
//     template public/sitemap.xml, the same one the single-document mode
//     uses.
type SitemapGroup struct {
	Name string       // child file name, e.g. "shoes.xml"
	URLs []SitemapURL // the URLs of the child file
}

// SitemapIndexBuilder assembles the groups rendered into a sitemap index.
//
// The builder receives the request Context so it can read request-scoped
// state and use c.Request.Context() for database or HTTP calls. Returning
// an error aborts the request with the framework's standard error mapping
// (500 + X-Log-Id).
//
// The builders run on every request — for the index document and for each
// child file. A builder that wants to avoid re-collecting the whole catalog
// reads the requested group through SitemapGroupName and queries only that
// group. The framework selects the group by name anyway, so a builder that
// ignores the hint stays correct and only pays for the extra work.
//
// The entries are used verbatim — the framework neither resolves a host nor
// normalizes a path. The template (or the builder) owns the scheme and host.
type SitemapIndexBuilder func(c *Context) ([]SitemapGroup, error)

// SitemapGroupName returns the group name the current request asks for.
// It is empty while the index document is rendered.
//
// It is a hint, not a contract: the framework always selects the group by
// name, so a builder that ignores it still produces a correct response —
// it only collects more than it needs.
//
//	app.SitemapIndex(func(c *xun.Context) ([]xun.SitemapGroup, error) {
//		name := xun.SitemapGroupName(c)
//		if name == "" {
//			// The index document: collect every group.
//		}
//		// ...
//	})
func SitemapGroupName(c *Context) string {
	return c.sitemapGroup
}

// SitemapIndex registers the GET /sitemap.xml route as a sitemap index
// document and the GET /sitemaps/{name} route serving one child sitemap
// file per group.
//
// The index document is rendered by the viewer named "sitemap-index.xml"
// (published by the static view engine when public/sitemap-index.xml
// exists) and lists one row per group: the row's Loc is the child's path
// ("/sitemaps/" plus the URL-escaped group name) and its LastMod is the
// most recent LastMod among the group's URLs. The child files are rendered
// by the viewer named "sitemap.xml" — the same public/sitemap.xml template
// the single-document mode uses.
//
// With no builder the content engine's entries are published as one group
// named "content.xml". With several builders their groups are concatenated
// in builder order, de-duplicated by name (first occurrence wins), and not
// re-sorted.
//
//	app.SitemapIndex(func(c *xun.Context) ([]xun.SitemapGroup, error) {
//		rows, err := db.QueryContext(c.Request.Context(),
//			"SELECT category, slug, updated_at FROM products ORDER BY category, id")
//		if err != nil {
//			return nil, err
//		}
//		defer rows.Close()
//
//		var out []xun.SitemapGroup
//		idx := map[string]int{}
//		for rows.Next() {
//			var cat, slug string
//			var updated time.Time
//			if err := rows.Scan(&cat, &slug, &updated); err != nil {
//				return nil, err
//			}
//			i, ok := idx[cat]
//			if !ok {
//				i = len(out)
//				idx[cat] = i
//				out = append(out, xun.SitemapGroup{Name: cat + ".xml"})
//			}
//			out[i].URLs = append(out[i].URLs, xun.SitemapURL{
//				Loc:     "/p/" + slug,
//				LastMod: updated.UTC().Format(time.RFC3339),
//			})
//		}
//		return out, rows.Err()
//	})
//
// Registration order decides precedence: calling SitemapIndex again
// replaces the previous registration, and a route registered with app.Get
// afterwards replaces it in turn. Replacing a route the framework does not
// own logs a warning.
//
// App.SitemapIndex and App.Sitemap are not meant to be combined: the later
// registration wins GET /sitemap.xml, and the child routes registered in
// index mode stay behind as orphans, because http.ServeMux cannot
// unregister a route.
//
// The routes answer 404 until the matching viewer is published —
// App.SitemapIndex does not require public/sitemap-index.xml (for the
// index document) or public/sitemap.xml (for the child files) to exist.
//
// A nil builder panics at registration: it is a programming error, and
// failing here keeps the stack trace pointing at the call site instead of
// at a request goroutine.
func (app *App) SitemapIndex(builders ...SitemapIndexBuilder) {
	for i, b := range builders {
		if b == nil {
			// Fail at registration: a nil builder would otherwise panic
			// inside the request goroutine, where the stack trace points
			// at the framework instead of the call site.
			panic(fmt.Sprintf("xun: SitemapIndex: nil builder at index %d", i))
		}
	}

	if len(builders) == 0 {
		builders = []SitemapIndexBuilder{defaultSitemapIndexBuilder}
	}

	if _, ok := app.routes[sitemapKey]; ok && !app.sitemapRouteOwned {
		app.logger.Warn("xun: sitemap index replaces the route " + sitemapKey +
			" registered outside the sitemap machinery")
	}

	// The index document takes over GET /sitemap.xml from the
	// single-document mode: the later registration wins.
	//
	// WithViewer() with no arguments clears the app's default handler
	// viewers: this route renders the named "sitemap-index.xml" viewer, it
	// does not negotiate one.
	if app.registerSitemapRoute(sitemapKey, sitemapIndexHandler(builders)) {
		// The route is ours now; a later app.Get("/sitemap.xml", ...) hands
		// ownership back to the user (see App.createHandler).
		app.sitemapRouteOwned = true
	}

	// The child route serves one <urlset> per group.
	app.registerSitemapRoute(sitemapChildPattern, sitemapChildHandler(builders))
}

// registerSitemapRoute registers a sitemap route on the app and reports
// whether it was registered.
//
// http.ServeMux panics when a pattern conflicts with one the mux already
// serves — including patterns the framework cannot see, like the
// "GET /{lang}/about" a page with a {lang} segment registers, or the
// method-less "/sitemaps/foo" a raw mux handler installs. net/http does
// not expose its conflict rules and the mux does not list its patterns,
// so the framework cannot enumerate them: the registration is done
// defensively and a conflict becomes a warning instead of a crash.
//
// Only ServeMux's conflict panic is handled — it carries an error value.
// Any other panic is passed on.
//
// A conflicting registration leaves the mux untouched (ServeMux detects the
// conflict before it changes its tree), so only the route entry the
// framework added has to be rolled back.
func (app *App) registerSitemapRoute(pattern string, hf HandleFunc) (registered bool) {
	_, existed := app.routes[pattern]

	defer func() {
		r := recover()
		if r == nil {
			return
		}

		err, ok := r.(error)
		if !ok {
			panic(r)
		}

		app.logger.Warn("xun: sitemap route "+pattern+" not registered",
			slog.String("route", pattern),
			slog.String("reason", err.Error()))

		if !existed {
			// The route was added to app.routes before the mux was asked
			// to serve it: roll the half-registered route back.
			delete(app.routes, pattern)
		}

		registered = false
	}()

	app.createHandler(pattern, hf, []RoutingOption{WithViewer()}, app)

	return true
}

// sitemapIndexHandler returns the handler serving GET /sitemap.xml as a
// sitemap index document.
//
// The viewer is resolved from app.viewers on every request — not once at
// registration time — so a template published (or hot-reloaded) after the
// route was registered is picked up without re-registering the route. When
// no viewer is published the route answers 404.
//
// The groups are collected before the viewer is looked up: a builder that
// fails is a server-side failure (500 + X-Log-Id) even when no template is
// published yet, so the error is not masked by the 404.
//
// The named viewer is rendered directly rather than through c.View: c.View
// falls back to Accept-header negotiation over the route's viewers, which
// would answer a JSON request with JSON instead of the sitemap index.
func sitemapIndexHandler(builders []SitemapIndexBuilder) HandleFunc {
	return func(c *Context) error {
		groups, err := collectSitemapGroups(c, builders)
		if err != nil {
			return err
		}

		rows := make([]SitemapURL, 0, len(groups))
		for _, g := range groups {
			rows = append(rows, SitemapURL{
				Loc:     sitemapChildURL(g.Name),
				LastMod: newestSitemapLastMod(g.URLs),
			})
		}

		// The index document is a document too: it must not exceed the
		// effective cap either.
		rows = truncateSitemapURLs(c, "sitemap index", rows)

		v, ok := c.App.viewers[sitemapIndexName]
		if !ok {
			c.WriteStatus(http.StatusNotFound)
			return nil
		}

		return v.Render(c, rows)
	}
}

// sitemapChildHandler returns the handler serving GET /sitemaps/{name} — the
// child sitemap file of one group.
//
// The requested group name is exposed to the builders through
// SitemapGroupName, but the framework always selects the group by name: a
// builder that ignores the hint still produces a correct response.
//
// A request for a name no group matches answers 404, as does a request
// while no viewer named "sitemap.xml" is published.
func sitemapChildHandler(builders []SitemapIndexBuilder) HandleFunc {
	return func(c *Context) error {
		// PathValue returns the unescaped segment, so the name matches the
		// group names the builders return directly.
		name := c.Request.PathValue("name")
		c.sitemapGroup = name

		groups, err := collectSitemapGroups(c, builders)
		if err != nil {
			return err
		}

		var urls []SitemapURL
		found := false
		for _, g := range groups {
			if g.Name == name {
				urls = g.URLs
				found = true
				break
			}
		}

		if !found {
			c.WriteStatus(http.StatusNotFound)
			return nil
		}

		v, ok := c.App.viewers[sitemapName]
		if !ok {
			c.WriteStatus(http.StatusNotFound)
			return nil
		}

		return v.Render(c, urls)
	}
}

// sitemapChildURL returns the URL path of a group's child sitemap file.
//
// The group name is URL-escaped so it stays inside one path segment. Two
// names need more than escaping: "." and ".." are path segments the server
// resolves away before routing (http.cleanPath), so a request for them
// would never reach the child route — their dots are escaped instead.
//
// A group without a name has no addressable path and is skipped by
// collectSitemapGroups.
func sitemapChildURL(name string) string {
	escaped := url.PathEscape(name)

	switch escaped {
	case ".":
		escaped = "%2E"
	case "..":
		escaped = "%2E%2E"
	}

	return sitemapChildPrefix + escaped
}

// collectSitemapGroups runs the builders in order and merges their groups.
//
// The groups are concatenated in builder order and de-duplicated by name,
// keeping the first occurrence: two groups with the same name would render
// the same child file twice, so the first one wins and the duplicate is
// logged. The result is not re-sorted.
//
// Each group's URLs are de-duplicated by the exact Loc string, keeping the
// first occurrence, and truncated to the effective cap. The de-duplication
// is per group: the same Loc in two groups is published in both files.
func collectSitemapGroups(c *Context, builders []SitemapIndexBuilder) ([]SitemapGroup, error) {
	var out []SitemapGroup
	seen := make(map[string]struct{})

	for _, b := range builders {
		groups, err := b(c)
		if err != nil {
			return nil, err
		}

		for _, g := range groups {
			if g.Name == "" {
				// A group without a name has no addressable path: the child
				// route matches a non-empty path segment.
				c.App.logger.Warn("xun: sitemap index: group without a name, skipped")
				continue
			}

			if _, ok := seen[g.Name]; ok {
				c.App.logger.Warn("xun: sitemap index: duplicate group name "+g.Name+", the first group wins",
					slog.String("name", g.Name))
				continue
			}
			seen[g.Name] = struct{}{}

			urls := dedupSitemapURLs(nil, make(map[string]struct{}, len(g.URLs)), g.URLs)
			g.URLs = truncateSitemapURLs(c, "sitemap group "+g.Name, urls)

			out = append(out, g)
		}
	}

	return out, nil
}

// newestSitemapLastMod returns the most recent LastMod among the entries.
//
// The values are RFC3339 timestamps, so they are compared as parsed times
// when they parse; a value that does not parse is compared as a string. The
// original string of the winning entry is returned.
func newestSitemapLastMod(urls []SitemapURL) string {
	newest := ""
	var newestAt time.Time

	for _, u := range urls {
		if u.LastMod == "" {
			continue
		}

		at, err := time.Parse(time.RFC3339, u.LastMod)
		if err != nil {
			at = time.Time{}
		}

		switch {
		case newest == "":
			newest, newestAt = u.LastMod, at
		case !at.IsZero() && !newestAt.IsZero():
			if at.After(newestAt) {
				newest, newestAt = u.LastMod, at
			}
		case u.LastMod > newest:
			// At least one of the two is not a timestamp: compare as
			// strings.
			newest, newestAt = u.LastMod, at
		}
	}

	return newest
}

// defaultSitemapIndexBuilder returns the content engine's entries as one
// group named "content.xml".
func defaultSitemapIndexBuilder(c *Context) ([]SitemapGroup, error) {
	return []SitemapGroup{{
		Name: "content.xml",
		URLs: c.App.SitemapURLs(),
	}}, nil
}
