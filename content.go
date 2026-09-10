package xun

import (
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// ContentView is the data exposed to templates via .Content for routes that
// were registered from a .md file.
//
// Seven fields:
//   - Title, Description are derived from Markdown semantics (AST walk).
//   - Path, Slug, Date come from the filesystem.
//   - Body is the rendered HTML produced by goldmark.
//   - Params holds arbitrary key/value pairs from a sibling .yaml sidecar
//     (see loadContentFile). Template authors define their own keys;
//     no schema is enforced.
type ContentView struct {
	Path        string         // "content/2026/deeper.md"
	Slug        string         // "2026/deeper"
	Title       string         // First # H1; empty if none
	Description string         // First blockquote (preferred) or top-level paragraph; empty if none
	Date        time.Time      // File mtime
	Body        template.HTML  // Rendered Markdown
	Params      map[string]any // Parsed from sibling .yaml, if present; nil otherwise
}

// contentRenderer wraps goldmark with GFM and exposes its parser so that
// Extract and Render share the same parse output.
type contentRenderer struct {
	md     goldmark.Markdown
	parser parser.Parser
}

// newContentRenderer constructs a renderer with GitHub-Flavored Markdown only.
func newContentRenderer() *contentRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)
	return &contentRenderer{
		md:     md,
		parser: md.Parser(),
	}
}

// extractFromAST walks an already-parsed AST and returns the first H1 title
// and the first blockquote-or-paragraph description.
//
// Rules:
//   - Title is the text of the first *ast.Heading with Level == 1.
//     Code blocks, fenced code blocks, and raw HTML are skipped via
//     WalkSkipChildren so "# inside them cannot become a title.
//   - Description is captured from the first paragraph that is either
//     directly under the document root or inside a blockquote (which
//     counts as a blockquote lede). Paragraphs nested inside lists or
//     other containers are ignored. Once a heading of any level appears
//     after the title, description capture stops.
//   - Both return empty strings if not found.
func (r *contentRenderer) extractFromAST(doc ast.Node, content []byte) (title, description string) {
	titleFound := false
	descriptionClosed := false

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		// Skip code blocks entirely (don't descend).
		if entering {
			switch n.(type) {
			case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.RawHTML:
				return ast.WalkSkipChildren, nil
			}
		}

		// Capture Title on exit (children's text has been accumulated).
		if !entering {
			if h, ok := n.(*ast.Heading); ok && h.Level == 1 && !titleFound {
				title = strings.TrimSpace(string(h.Lines().Value(content)))
				titleFound = true
			}
			return ast.WalkContinue, nil
		}

		// After title: any further heading closes description capture.
		if titleFound {
			if h, ok := n.(*ast.Heading); ok && h.Level >= 2 {
				descriptionClosed = true
			}
		}

		// Capture Description after Title is known.
		if titleFound && !descriptionClosed && description == "" {
			if p, ok := n.(*ast.Paragraph); ok {
				switch p.Parent().(type) {
				case *ast.Document, *ast.Blockquote:
					description = strings.TrimSpace(string(p.Lines().Value(content)))
				}
			}
		}
		return ast.WalkContinue, nil
	})

	return title, description
}

// Extract parses the content and returns the first H1 title and the first
// blockquote-or-paragraph description. See extractFromAST for the rules.
//
// Prefer Process when the caller also needs the rendered body: Process
// parses the AST once and reuses it for both extraction and rendering.
func (r *contentRenderer) Extract(content []byte) (title, description string) {
	doc := r.parser.Parse(text.NewReader(content))
	return r.extractFromAST(doc, content)
}

// Render converts Markdown bytes into template.HTML. goldmark has already
// escaped the output, so html/template will not re-escape it.
//
// The output buffer is taken from the package-level BufPool to avoid a fresh
// bytes.Buffer allocation per call. The bytes.Buffer.Reset called by Put
// preserves the underlying capacity, so the pool warms up to the largest
// rendered size seen in flight and reuses that storage across calls.
func (r *contentRenderer) Render(content []byte) (template.HTML, error) {
	buf := BufPool.Get()
	defer BufPool.Put(buf)
	if err := r.md.Convert(content, buf); err != nil {
		return "", err
	}
	// template.HTML is `type HTML string`; the []byte→string conversion
	// copies. We snapshot the bytes first because BufPool.Put (via defer)
	// will Reset the buffer, invalidating the slice view. The defer Put
	// runs after this function returns, by which point the string is
	// already independent of the pooled buffer's backing array.
	b := buf.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return template.HTML(out), nil
}

// renderAST renders an already-parsed AST to template.HTML using the
// package-level BufPool. The body is snapshotted into a fresh slice before
// the buffer is returned to the pool, so the returned template.HTML is
// independent of BufPool state.
func (r *contentRenderer) renderAST(doc ast.Node, source []byte) (template.HTML, error) {
	buf := BufPool.Get()
	defer BufPool.Put(buf)
	if err := r.md.Renderer().Render(buf, source, doc); err != nil {
		return "", err
	}
	b := buf.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return template.HTML(out), nil
}

// parseAndExtract parses the markdown content and returns the parsed AST
// together with the extracted title and description. It exists so callers
// (loadContentFile in particular) can share one parse between metadata
// extraction and a later render step.
func (r *contentRenderer) parseAndExtract(content []byte) (doc ast.Node, title, description string) {
	doc = r.parser.Parse(text.NewReader(content))
	title, description = r.extractFromAST(doc, content)
	return
}

// Process parses the Markdown content once and returns the extracted
// metadata together with the rendered HTML body. It exists so the typical
// loadContentFile path does not pay for two parses (one for Extract, one for
// Convert/Render).
//
// The returned title/description follow the same rules as extractFromAST.
// The body uses the package-level BufPool so the bytes.Buffer backing array
// is reused across calls.
func (r *contentRenderer) Process(content []byte) (title, description string, body template.HTML, err error) {
	doc, title, description := r.parseAndExtract(content)
	body, err = r.renderAST(doc, content)
	return
}

// buildContentView derives slug/date from filesystem info and fills the
// non-AST fields of ContentView. It does not touch the Markdown content;
// callers pass the title/description/body that Process (or extractFromAST)
// already produced.
func buildContentView(mdPath string, fi fs.FileInfo, contentDir, title, description string, body template.HTML) ContentView {
	// Slug: mdPath minus content directory prefix and ".md" extension.
	slug := strings.TrimSuffix(mdPath, ".md")
	if contentDir != "" {
		slug = strings.TrimPrefix(slug, contentDir+"/")
	}

	// Date: file mtime if available.
	var date time.Time
	if fi != nil {
		date = fi.ModTime()
	}

	return ContentView{
		Path:        mdPath,
		Slug:        slug,
		Title:       title,
		Description: description,
		Date:        date,
		Body:        body,
	}
}
