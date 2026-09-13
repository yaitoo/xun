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
// templates (via c.View(data, "public/sitemap.xml")) call this —
// they cannot drift.
func (app *App) SitemapURLs(opts SitemapOptions, c *Context) []SitemapURL {
	out := make([]SitemapURL, 0, len(app.contentViews))
	for _, cv := range app.contentViews {
		if opts.Filter != nil && !opts.Filter(cv) {
			continue
		}
		u := SitemapURL{Loc: buildSitemapLoc(c, cv)}
		if !cv.Date.IsZero() {
			u.LastMod = cv.Date.UTC().Format(time.RFC3339)
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out
}

// buildSitemapLoc resolves the absolute URL for a ContentView from the
// current request. Falls back to a root-relative path when no request
// context is available (e.g., background callers).
func buildSitemapLoc(c *Context, cv *ContentView) string {
	if c == nil || c.Request == nil {
		return "/" + strings.TrimLeft(cv.Slug, "/")
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/" + strings.TrimLeft(cv.Slug, "/")
}