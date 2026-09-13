package xun

import (
	"sort"
	"strings"
	"time"
)

// SitemapURL is one <url> entry in a sitemap.xml. The framework only
// surfaces content engine routes — see App.SitemapURLs for the rationale.
//
// # Field naming
//
// Loc and LastMod mirror the sitemap.xml element names
// (<loc>https://...</loc> and <lastmod>RFC3339</lastmod>) on purpose:
// templates can write `{{ .Loc }}` / `{{ .LastMod }}` without consulting
// a cheat sheet, and the connection between the field and the rendered
// element is obvious from the names alone.
//
//   - Loc     — URL *path* (e.g. "/content/post"), not a full URL.
//   - LastMod — RFC3339 timestamp of the source file's mtime
//               (ContentView.LastMod, populated via fs.Stat(...).ModTime()
//               at load time). Empty when Stat failed — guard with
//               `{{ if .LastMod }}`.
//
// Both fields are deliberately minimal:
//
//   - Loc is a path because the framework does not know which scheme +
//     host your site is deployed under. The host prefix is the template's
//     job — `<loc>https://example.com{{ .Loc }}</loc>`. Putting the host
//     into Loc would force callers to thread scheme/host through every
//     call site and make off-the-shelf `c.App.SitemapURLs()` callers
//     unable to do anything useful.
//
//   - LastMod is just file mtime. There is no "created" timestamp, no
//     scheduled-publish date, no update-vs-create distinction. Sites
//     that need richer metadata should compute it at .md parse time
//     (goldmark AST, sidecar .yaml) and surface it via their own helper.
//
// changefreq and priority are deliberately NOT fields here:
//
//   - The framework has no reliable signal for either (a .md file gives
//     no hint whether its author updates it daily or yearly).
//   - Google has publicly stated they ignore both fields.
//
// If you need changefreq / priority, extend SitemapURL at the template
// layer by post-processing App.SitemapURLs output, or write a custom
// handler that walks app.contentViews.
//
// # Filtering
//
// SitemapURL has no Filter hook. If you need to drop drafts, encode the
// signal into a convention your template understands (URL prefix,
// sidecar .yaml that your template reads via a custom funcMap, etc.).
// Putting a Filter on App.SitemapURLs would mean the framework tracks
// state on every .md — the docs already recommend keeping the
// framework's surface minimal.
type SitemapURL struct {
	Loc     string // URL path, e.g. "/content/post". Host/scheme are template-side.
	LastMod string // RFC3339; empty when the source has no trackable mtime.
}

// SitemapURLs returns the URL entries derived from app.contentViews,
// sorted by Loc for stable output. Safe to call anywhere (no Context
// dependency — Loc is a path, not an absolute URL).
//
// # Scope: content engine only
//
// SitemapURLs walks app.contentViews, not app.routes. This is an
// intentional, narrow scope:
//
//   - content/*.md (with a bubble-up template) is the framework's
//     managed content surface. Every entry has a route, a slug, a
//     timestamp, and a known URL pattern. The framework owns the
//     data and the lifecycle; emitting them in the sitemap is
//     straightforward and reliable.
//
//   - pages/*.html and public/*.html are user-authored. The framework
//     has no mtime, no slug convention, no semantic metadata for them.
//     The user knows which of their pages are indexable; if they want
//     them in the sitemap, they add the URLs to their template by
//     hand (or by extending SitemapURL and post-processing).
//
//   - app.Get routes are user handlers. By the framework's
//     classification they aren't "pages" — including them in a
//     sitemap (a content-discovery format) by default would surprise
//     every caller who set up an API or webhook endpoint.
//
// This narrow scope is the framework's way of staying out of the
// user's way. The framework guarantees: every URL in App.SitemapURLs
// resolves to a working page on this server. The user decides
// whether that's the right set for their sitemap; if not, they
// post-process.
//
// # Path construction
//
// URL paths are derived from the routes-map pattern key (e.g.
// "GET /content/post" → "/content/post"), not from cv.Slug. cv.Slug
// is path-relative to the content directory; the actual route includes
// the content-dir prefix. With WithContent("blog") and blog/post.md,
// the route is /blog/post, but cv.Slug would be "post" — emitting
// /post would 404.
//
// Index pages get a trailing slash. blog/index.md → pattern
// "GET /blog/{$}" → canonical "/blog/" (matches the route exactly;
// without the slash, /blog 307-redirects to /blog/, wasting one
// round-trip per crawler fetch).
//
// Patterns with {var} segments have no concrete URL and are skipped.
//
// # Orphan skipping
//
// Entries without a registered route are skipped. loadContentFile
// writes to contentViews before checking for a bubble-up template,
// so orphan entries exist when a .md has no .tpl sibling / ancestor.
// A sitemap URL for an unrouted page is worse than no URL.
//
// # Output guarantee
//
// Every Loc returned here resolves to a working page on this server.
// Sort is by Loc for stable output (sitemaps don't care about order
// but stable order helps tests and review).
func (app *App) SitemapURLs() []SitemapURL {
	out := make([]SitemapURL, 0, len(app.contentViews))
	for pattern, cv := range app.contentViews {
		if _, hasRoute := app.routes[pattern]; !hasRoute {
			continue
		}
		p := strings.TrimPrefix(pattern, "GET ")
		switch {
		case strings.HasSuffix(p, "/{$}"):
			p = strings.TrimSuffix(p, "/{$}") + "/"
		case strings.ContainsAny(p, "{}"):
			continue
		}
		u := SitemapURL{Loc: p}
		if !cv.LastMod.IsZero() {
			u.LastMod = cv.LastMod.UTC().Format(time.RFC3339)
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out
}