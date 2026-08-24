package xun

import (
	"bytes"
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
// Six fields:
//   - Title, Description are derived from Markdown semantics (AST walk).
//   - Path, Slug, Date come from the filesystem.
//   - Body is the rendered HTML produced by goldmark.
type ContentView struct {
	Path        string         // "content/2026/deeper.md"
	Slug        string         // "2026/deeper"
	Title       string         // First # H1; empty if none
	Description string         // First blockquote (preferred) or top-level paragraph; empty if none
	Date        time.Time      // File mtime
	Body        template.HTML  // Rendered Markdown
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

// Extract walks the parsed AST once and returns the first H1 title and the
// first blockquote-or-paragraph description.
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
func (r *contentRenderer) Extract(content []byte) (title, description string) {
	doc := r.parser.Parse(text.NewReader(content))
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

// Render converts Markdown bytes into template.HTML. goldmark has already
// escaped the output, so html/template will not re-escape it.
func (r *contentRenderer) Render(content []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(content, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// extractContentView combines filesystem info and Markdown semantics into a
// ContentView. The Body field is left zero; the caller fills it after rendering.
//
// Title and Description are derived from the Markdown AST. If the document
// has no # H1, Title is left empty (callers should guard with {{if .Title}}).
func extractContentView(mdPath string, content []byte, fi fs.FileInfo, contentDir string, r *contentRenderer) ContentView {
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

	// Title / Description: AST walk.
	title, description := r.Extract(content)

	return ContentView{
		Path:        mdPath,
		Slug:        slug,
		Title:       title,
		Description: description,
		Date:        date,
		Body:        template.HTML(""),
	}
}