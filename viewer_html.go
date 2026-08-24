package xun

import (
	"net/http"
	"strings"
)

// HtmlViewer is a viewer that renders a html template.
//
// It uses the `HtmlTemplate` type to render a template.
// The template is loaded from the file system when the viewer is created.
// The `Render` method renders the template with the given data and writes the
// result to the http.ResponseWriter.
type HtmlViewer struct {
	template *HtmlTemplate
}

var htmlViewerMime = &MimeType{Type: "text", SubType: "html"}

// MimeType returns the MIME type of the HTML content.
//
// This implementation returns "text/html".
func (*HtmlViewer) MimeType() *MimeType {
	return htmlViewerMime
}

// Render renders the template with the given data and writes the result to the http.ResponseWriter.
//
// This implementation uses the `HtmlTemplate.Execute` method to render the template.
// The rendered result is written to the http.ResponseWriter.
func (v *HtmlViewer) Render(ctx *Context, data any) error { // skipcq: RVV-B0012
	var err error
	ctx.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ctx.Request.Method != http.MethodHead {
		buf := BufPool.Get()
		defer BufPool.Put(buf)

		vm := ViewModel{TempData: ctx.TempData, Data: data}
		if ctx.App != nil {
			if cv := ctx.App.lookupContent(ctx.Routing.Pattern); cv != nil {
				vm.Content = cv
			}
			vm.Breadcrumb = buildBreadcrumb(ctx)
		}

		err = v.template.Execute(buf, vm)
		if err != nil {
			return err
		}
		_, err = buf.WriteTo(ctx.Response)
	}
	return err
}

// buildBreadcrumb walks the request URL and produces the ancestor chain for
// templates. See docs/breadcrumb.md §2 for the full algorithm and rationale.
//
// The returned slice is ordered top-to-bottom: items[0] is the root
// ("Home"), items[len-1] is the current page with Last == true. nil on the
// root path (GET /{$}).
func buildBreadcrumb(ctx *Context) []BreadcrumbItem {
	if ctx.Request == nil || ctx.Request.URL == nil {
		return nil
	}

	path := strings.Trim(ctx.Request.URL.Path, "/")
	if path == "" {
		return nil
	}

	segs := strings.Split(path, "/")
	items := make([]BreadcrumbItem, 0, len(segs)+1)

	items = append(items, BreadcrumbItem{Path: "/", Name: "Home"})

	for i, seg := range segs {
		prefix := "/" + strings.Join(segs[:i+1], "/")
		// Ancestor items are looked up by URL-derived pattern because the
		// ancestor's content view, if any, was registered against the
		// concrete URL. The current page's Routing.Pattern may contain
		// braces for dynamic .md routes (e.g. "GET /blog/{slug}") and
		// must be preferred so dynamic .md ancestors keep their H1.
		pattern := "GET " + prefix
		if i == len(segs)-1 && ctx.Routing.Pattern != "" {
			pattern = ctx.Routing.Pattern
		}

		title := ""
		if ctx.App != nil {
			if cv := ctx.App.lookupContent(pattern); cv != nil && cv.Title != "" {
				title = cv.Title
			}
		}

		items = append(items, BreadcrumbItem{
			Path:  prefix,
			Name:  seg,
			Title: title,
		})
	}

	items[len(items)-1].Last = true
	return items
}
