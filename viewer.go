package xun

import (
	"net/http"
)

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
	Render(w http.ResponseWriter, r *http.Request, data any) error
}
