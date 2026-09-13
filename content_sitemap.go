package xun

import (
	"sort"
	"strings"
	"time"
)

// SitemapURL is one <url> entry in a sitemap.xml.
//
// Only Loc and LastMod are surfaced: changefreq and priority are not
// derivable from a Markdown file (the framework has no signal for either),
// and Google has publicly stated they ignore them. If you need them,
// add them at the template layer by extending SitemapURL in your own code
// or by post-processing App.SitemapURLs output.
type SitemapURL struct {
	Loc     string
	LastMod string // RFC3339; empty when the source file has no mtime
}

// SitemapOptions controls how App.SitemapURLs derives URL entries from
// the loaded contentViews map. The zero value is usable.
type SitemapOptions struct {
	// Filter lets callers drop ContentViews (typical use: skip drafts).
	// Returning false excludes the entry. Default (nil) keeps every entry.
	Filter func(*ContentView) bool
}

// SitemapURLs returns the URL entries derived from app.contentViews,
// sorted by Loc for stable output. Safe to call inside a handler.
//
// This is the single source of truth for "what URLs does this site
// expose?". Both the default sitemap handler (registered by
// StaticViewEngine for public/sitemap.xml) and user-authored sitemap
// templates (via c.View(data, "sitemap.xml")) call this —
// they cannot drift.
//
// Two corrections keep the URLs honest:
//
//   - The URL path is derived from the routes-map pattern key (e.g.
//     "GET /blog/post"), NOT from cv.Slug. cv.Slug is path-relative
//     to the content directory; the actual route includes the content-dir
//     prefix. With WithContent("blog") and blog/post.md, the route is
//     /blog/post, but cv.Slug would be "post" — emitting /post 404s.
//
//   - Entries without a registered route are skipped. loadContentFile
//     writes to contentViews before checking for a bubble-up template,
//     so orphan entries exist when a .md has no .tpl sibling / ancestor.
//     A sitemap URL for an unrouted page is worse than no URL.
func (app *App) SitemapURLs(opts SitemapOptions, c *Context) []SitemapURL {
	out := make([]SitemapURL, 0, len(app.contentViews))
	for pattern, cv := range app.contentViews {
		// Skip orphans: .md with no bubble-up template never gets a route.
		if _, hasRoute := app.routes[pattern]; !hasRoute {
			continue
		}
		if opts.Filter != nil && !opts.Filter(cv) {
			continue
		}
		// pattern is e.g. "GET /blog/post" or "GET /blog/{$}" for index.md.
		// Strip method prefix and /{$} suffix to get a concrete URL path.
		// Skip patterns containing '{' / '}' — no concrete URL is possible.
		p := strings.TrimPrefix(pattern, "GET ")
		p = strings.TrimSuffix(p, "/{$}")
		if strings.ContainsAny(p, "{}") {
			continue
		}
		u := SitemapURL{Loc: buildSitemapLoc(c, p)}
		if !cv.Date.IsZero() {
			u.LastMod = cv.Date.UTC().Format(time.RFC3339)
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out
}

// buildSitemapLoc resolves an absolute URL from a URL path and the current
// request. Falls back to a root-relative path when no request context is
// available (e.g., background callers).
func buildSitemapLoc(c *Context, path string) string {
	if c == nil || c.Request == nil {
		return path
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + path
}