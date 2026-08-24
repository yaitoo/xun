package xun

// BufPool is a pool of *bytes.Buffer for reuse to reduce memory alloc.
//
// It is used by the Viewer to render the content.
// The pool is created with a size of 100, but you can change it by setting the
// BufPool variable before creating any Viewer instances.
var BufPool *BufferPool

func init() {
	BufPool = NewBufferPool(100)
}

// Viewer is the interface that wraps the minimum set of methods required for
// an effective viewer.
type Viewer interface {
	MimeType() *MimeType
	Render(ctx *Context, data any) error
}

// BreadcrumbItem is one node in the breadcrumb chain rendered into templates.
//
//   - Path:  the URL prefix leading to this node, including leading slash.
//     The root item has Path = "/".
//   - Name:  the link label. For non-root items this is the last segment of
//     Path verbatim (no transformation). The root item has the fixed
//     Name "Home".
//   - Title: hover hint, sourced from ContentView.Title (Markdown H1).
//     Empty for non-Markdown ancestors.
//   - Last:  true on the trailing item (current page).
type BreadcrumbItem struct {
	Path  string
	Name  string
	Title string
	Last  bool
}

// ViewModel holds the context and associated data for rendering.
type ViewModel struct {
	TempData map[string]any
	Data     any
	Content  *ContentView // optional, populated when the route was registered from a .md file

	// Breadcrumb is the ancestor chain for the current request URL, top to
	// bottom, with the current page as the trailing element (Last == true).
	// nil on the root path (GET /{$}). See docs/breadcrumb.md.
	Breadcrumb []BreadcrumbItem
}
