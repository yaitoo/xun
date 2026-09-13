package xun

import (
	"sort"
	"strings"
	"time"
)

// SitemapURL is one <url> entry in a sitemap.xml.
//
// Loc is a URL *path* (e.g. "/about", "/blog/", "/content/post") — the
// framework does not know which scheme + host your site is deployed
// under, so the host prefix is the template's job:
//
//	<loc>https://example.com{{ .Loc }}</loc>
//
// Putting the host into Loc would force callers to thread scheme/host
// through every call site and make off-the-shelf `c.App.SitemapURLs()`
// callers unable to do anything useful.
//
// Only Loc and LastMod are surfaced: changefreq and priority are not
// derivable from a Markdown file (the framework has no signal for either),
// and Google has publicly stated they ignore them. If you need them,
// add them at the template layer by extending SitemapURL in your own code
// or by post-processing App.SitemapURLs output.
//
// Filtering is intentionally not exposed here: drop entries in your
// sitemap.xml template by checking whatever fields you encode on SitemapURL
// or by inspecting app.routes / app.contentViews via a custom handler.
type SitemapURL struct {
	Loc     string // URL path, e.g. "/about". Host/scheme are template-side.
	LastMod string // RFC3339; empty when the source has no trackable mtime.
}

// SitemapURLs returns every indexable URL the App knows about, drawn from
// exactly three sources:
//
//  1. pages/**/*.html        — registered by HtmlViewEngine.loadPage
//     (HtmlViewer). Path is the URL with .html stripped and /{$} index
//     marker canonicalised to a trailing slash.
//
//  2. public/**/*.html       — registered by StaticViewEngine.handle
//     (FileViewer). Path is the URL with .html / index.html stripping
//     handled by HandleFile. Only routes whose path ends in "/" or
//     ".html" are included — CSS / JS / images / fonts are skipped.
//
//  3. contentViews           — every entry that has a registered route
//     (orphans without a bubble-up template are excluded). The route
//     is an HtmlViewer wrapping the bubble-up template.
//
// Routes registered via app.Get with their own handler (JsonViewer in
// r.Viewers[0]) are excluded — they aren't a "page" by the framework's
// classification, even if they happen to serve HTML.
//
// LastMod is only set for content engine routes (the file mtime is
// tracked in ContentView.Date). pages/* and public/* have no trackable
// mtime in the framework, so LastMod stays empty — guard in your
// template with `{{ if .LastMod }}`.
func (app *App) SitemapURLs() []SitemapURL {
	out := make([]SitemapURL, 0, len(app.routes))
	for pattern, r := range app.routes {
		if !strings.HasPrefix(pattern, "GET ") {
			continue
		}
		if pattern == "GET /sitemap.xml" {
			continue
		}

		p := strings.TrimPrefix(pattern, "GET ")
		switch {
		case strings.HasSuffix(p, "/{$}"):
			// Index page canonical URL has a trailing slash. Without it,
			// /blog 307-redirects to /blog/, costing one round-trip per
			// crawler fetch.
			p = strings.TrimSuffix(p, "/{$}") + "/"
		case strings.ContainsAny(p, "{}"):
			// Variable segment — no concrete URL possible.
			continue
		}

		if _, isContent := app.contentViews[pattern]; isContent {
			// Source 3: content engine route.
		} else if len(r.Viewers) > 0 {
			switch r.Viewers[0].(type) {
			case *HtmlViewer:
				// Source 1: pages/*.html.
			case *FileViewer:
				// Source 2: public/**/*.html. Only keep paths that look
				// like HTML pages (end with "/" from index.html handling,
				// or with ".html" for non-index .html files). Everything
				// else in public/ is a static asset.
				if !(strings.HasSuffix(p, "/") || strings.HasSuffix(p, ".html")) {
					continue
				}
			default:
				// JsonViewer (user app.Get), XmlViewer, etc. — not a page.
				continue
			}
		} else {
			continue
		}

		u := SitemapURL{Loc: p}
		// LastMod is meaningful only for content engine routes — pages/*
		// and public/* have no tracked mtime in the framework.
		if cv, ok := app.contentViews[pattern]; ok && !cv.Date.IsZero() {
			u.LastMod = cv.Date.UTC().Format(time.RFC3339)
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out
}