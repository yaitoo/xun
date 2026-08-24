# Breadcrumb Design

> Status: DESIGN — Phase 1 implementation pending
> Companion to `content.md`. Extends `HtmlViewEngine` with ancestor-aware navigation data.

This document describes how xun surfaces a breadcrumb chain to HTML templates. The mechanism is deliberately minimal: it walks the request URL, pairs each ancestor with its Markdown H1 (when available), and exposes the chain as `ViewModel.Breadcrumb`. There is no new file convention, no new option, no new template function — only data.

---

## Section 0 — Design Principles

### Principle 0.1 — Data only, no UI coupling

The breadcrumb feature produces a `[]BreadcrumbItem` slice and nothing else. No template function, no JSON-LD helper, no schema.org generator, no `WithBreadcrumb(false)` toggle. Templates and handlers consume the slice directly. Anything that resembles rendering or presentation stays in user code.

### Principle 0.2 — No new metadata infrastructure

There is no `pageMeta` map, no `<!--meta:K=V-->` comment parser, no `WithPageMeta` routing option. The breadcrumb is computed from two sources that already exist:

1. `ctx.Request.URL.Path` (URL segments).
2. `App.contentViews` (Markdown H1s, populated by the content engine per `content.md`).

If a future feature needs cross-route page metadata, it builds its own store. Breadcrumb does not introduce one.

### Principle 0.3 — `Name` and `Title` are distinct

- `Name` is the **link label** rendered inside `<a>`. It comes from the URL segment, verbatim, with no transformation.
- `Title` is the **hover tooltip** rendered as the `title` attribute. It comes from `ContentView.Title` (the Markdown H1) and is empty for non-Markdown ancestors.

These two fields answer different questions: "what does the user see?" versus "what extra context does the page offer on hover?". Conflating them — as a single "title" or "label" field — loses information.

### Principle 0.4 — Current page is the last item

The chain includes the current page as the trailing element with `Last == true`. Templates identify the current page by this flag, not by string comparison against the request URL (which is not visible inside templates anyway). "父辈" in the original discussion is loose: in this design, the array means "the full breadcrumb chain", ancestors in the strict-graph sense.

### Principle 0.5 — Root path produces an empty chain

`GET /{$}` has no ancestors. `vm.Breadcrumb` is `nil`. Templates guard with `{{if .Breadcrumb}}` or `{{with .Breadcrumb}}` and degrade gracefully.

---

## Section 1 — Data Structure

```go
// BreadcrumbItem is one node in the breadcrumb chain rendered into templates.
//
//   Path  — the URL prefix leading to this node, including leading slash.
//           The root item has Path = "/".
//   Name  — the link label, equal to the last segment of Path verbatim.
//           The root item has Name = "Home" (fixed constant).
//   Title — hover hint, sourced from ContentView.Title (Markdown H1).
//           Empty for non-Markdown ancestors.
//   Last  — true on the trailing item (current page). Templates use this
//           to decide between <a> and <span>, instead of comparing paths.
type BreadcrumbItem struct {
    Path  string
    Name  string
    Title string
    Last  bool
}

// ViewModel gains one field. Existing TempData, Data, Content are unchanged.
type ViewModel struct {
    TempData   map[string]any
    Data       any
    Content    *ContentView
    Breadcrumb []BreadcrumbItem // top → bottom, trailing = current page; nil on root
}
```

The slice is ordered top-to-bottom: `items[0]` is the root, `items[len-1]` is the current page. JSON-LD emission (out of scope for this document) iterates the slice in order.

---

## Section 2 — Construction Algorithm

`HtmlViewer.Render` builds `vm.Breadcrumb` immediately after the existing `vm.Content` lookup, before `template.Execute`:

```go
path := strings.Trim(ctx.Request.URL.Path, "/")
if path == "" {
    vm.Breadcrumb = nil
    return // root: no chain
}

segs := strings.Split(path, "/")
items := make([]BreadcrumbItem, 0, len(segs)+1)

// Root: fixed label, no metadata lookup.
items = append(items, BreadcrumbItem{Path: "/", Name: "Home"})

// Ancestors + current page, in URL order.
for i, seg := range segs {
    prefix := "/" + strings.Join(segs[:i+1], "/")
    pattern := "GET " + prefix

    title := ""
    if cv := ctx.App.lookupContent(pattern); cv != nil && cv.Title != "" {
        title = cv.Title
    }

    items = append(items, BreadcrumbItem{
        Path:  prefix,
        Name:  seg,
        Title: title,
    })
}

items[len(items)-1].Last = true
vm.Breadcrumb = items
```

Three notes:

1. **`Name` is the raw segment.** No `humanize`, no `title-case`, no decoding. "deep-dive" stays "deep-dive". Templates or template funcs may post-process if they want; the framework does not assume an English kebab-case convention.
2. **Root `Name` is hardcoded to `"Home"`.** There is no override path in this design. Localized sites translate in templates or supply a custom viewer. See Section 5 for the rationale.
3. **`Title` is set per ancestor independently.** Each prefix performs its own `lookupContent`, so a Markdown post at `/blog/2026/x.md` carries its own H1 while its parent `index.md` carries a different one.

---

## Section 3 — File Changes

Two files, additive only.

| File | Change |
|---|---|
| `viewer.go` | Add `BreadcrumbItem` type. Add `Breadcrumb []BreadcrumbItem` field to `ViewModel`. |
| `viewer_html.go` | In `Render`, after the existing `lookupContent` branch, build `vm.Breadcrumb` per Section 2. |

No other files change. `template_html.go`, `viewengine_html.go`, `routing_option.go`, `app.go`, `option.go`, and `funcmap.go` are untouched.

---

## Section 4 — Template Usage

A layout-level component renders the chain. The minimum is:

```html
<nav aria-label="breadcrumb"><ol>
{{range .Breadcrumb}}
<li>{{if .Last}}{{.Name}}{{else}}<a href="{{.Path}}"{{with .Title}} title="{{.}}"{{end}}>{{.Name}}</a>{{end}}</li>
{{end}}
</ol></nav>
```

The `Last` branch renders the current page as a plain `<li>` (no link, no `title` attribute even when empty). The non-`Last` branch renders an `<a>` whose `title` attribute is omitted when `Title` is empty thanks to `{{with}}`.

For sites that want richer markup (schema.org JSON-LD, microdata, custom CSS classes), iterate the same slice and emit whatever structure the design system requires. No framework helper is involved.

---

## Section 5 — Deliberate Non-Goals

The following are out of scope by design. Each is one bullet because none merits a paragraph until a real use case demands it.

- **No page-meta comment parser.** `<!--meta:K=V-->` is not introduced. `pageMeta` does not exist.
- **No `WithPageMeta` routing option.** Code-defined routes that need breadcrumb overrides manipulate `vm.Breadcrumb` in the handler (see Section 6).
- **No `WithNavigation` integration.** `WithNavigation` continues to populate `Routing.Options.metadata` for code-side consumption. It is not mirrored into a breadcrumb store. If a future feature needs the navigation tree, that is a separate design document.
- **No `Home` override.** Root `Name` is the literal string `"Home"`. Localization or rename is the user's responsibility, handled in the layout template or via a custom viewer.
- **No `humanize` on segments.** "deep-dive" stays "deep-dive". The framework does not encode assumptions about URL conventions.
- **No JSON-LD helper.** Templates or a future extension emit schema.org markup.
- **No `WithBreadcrumb(false)` toggle.** To suppress the chain, render `{{if false}}` around the `range` or omit the layout component. The framework always computes it; computation is one map lookup per ancestor plus one allocation.
- **No dynamic-segment special case.** `/user/{id}` produces a trailing item whose `Name` is the literal ID (`"42"`). Handlers needing a friendly name override in `vm.Breadcrumb` after construction (Section 6).
- **No multi-host awareness.** `@abc.com/...` pages are not addressed in this design.

---

## Section 6 — Handler Overrides

When the default segment-derived `Name` is wrong (dynamic routes, renamed slugs, i18n), the handler adjusts the slice after the viewer has built it. Two patterns:

**Mutate the trailing item** (typical for `/user/{id}`):

```go
app.Get("/user/{id}", func(c *Context) error {
    u := loadUser(c.Request.PathValue("id"))
    // The viewer ran before us? No — Render runs after the handler returns.
    // We can't mutate vm.Breadcrumb post-hoc; instead set a TempData hint
    // and let the layout apply it.
    c.Set("currentName", u.DisplayName)
    return c.View(u)
})
```

Layout consumes the hint:

```html
{{$current := ""}}
{{with .TempData.currentName}}{{$current = .}}{{end}}
{{range .Breadcrumb}}
<li>{{if .Last}}{{if $current}}{{$current}}{{else}}{{.Name}}{{end}}{{else}}<a href="{{.Path}}">{{.Name}}</a>{{end}}</li>
{{end}}
```

This keeps the framework's contract intact — `vm.Breadcrumb` is always populated, never partially — while letting handlers localize the current label through `TempData`.

**Rebuild the chain** (rare; only when ancestors themselves differ):

For now, this is not supported by the framework API. If a real case emerges, a `Context.RebuildBreadcrumb(...)` helper can be added without breaking this design — `BreadcrumbItem` is exported, the field is exported, and the build logic is localized in `HtmlViewer.Render`.

---

## Section 7 — Test Plan

Tests live next to existing engine tests, following the `*_test.go` convention. Each row below is one `t.Run` subtest in a single `TestBreadcrumb` function.

| Input | URL | Expected chain |
|---|---|---|
| Root page | `/` | `Breadcrumb == nil` |
| Single segment | `/blog` | `[{Path:"/", Name:"Home"}, {Path:"/blog", Name:"blog", Title:"", Last:true}]` |
| Multi segment, no markdown | `/blog/2026/x.html` (assuming no matching `.md`) | trailing item has `Title == ""` |
| Multi segment, markdown ancestor | `/blog/2026/x.md` where H1 = "深入 Xun" | trailing item has `Title == "深入 Xun"`; intermediate `/blog/2026/index.md` items carry their own H1s when present |
| Chinese URL segment | `/blog/深入探索` | `Name == "深入探索"` (raw, no transliteration) |
| Dynamic segment | `/user/42` | trailing item has `Name == "42"`, `Last == true`, `Title == ""` |
| Trailing slash normalization | `/blog/` (Sermux canonicalizes) | behaves identically to `/blog` |
| Hot reload | Edit `.md` H1, refetch | new `Title` reflects new H1 (re-derives via existing content engine reload path) |

Coverage for HTML escaping lives in the existing template tests; `BreadcrumbItem` fields are plain strings and `html/template` handles them automatically. No special escaping tests are needed.

---

## Section 8 — Compatibility

This feature is fully additive:

- `ViewModel` gains one field; existing three-field reads (template paths that use only `.TempData`, `.Data`, `.Content`) keep working.
- `HtmlViewer.Render` changes shape but not signature.
- No public API is renamed or removed.
- No file convention is introduced or modified.
- `WithNavigation`, `WithMetadata`, `Routing.Options.Get`, `WithViewer`, and the existing `RoutingOptions` accessors remain untouched.

The change passes the AGENTS.md §20 "safe-change checklist": minimal, additive, preserves MIME/route/hot-reload semantics, no default-behavior change for sites that do not read `.Breadcrumb`.
