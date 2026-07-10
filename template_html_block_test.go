package xun

import (
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestBlockWithoutDefinition tests that {{block}} works correctly when the block is not defined in the page
// This is the core issue from https://github.com/yaitoo/xun/issues/109
func TestBlockWithoutDefinition(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{block "optional" .}}default content{{end}}</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->{{define "content"}}page content{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	// Load layout first
	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	// Load page that uses layout but doesn't define the optional block
	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Execute should not error - the block should render with default content
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "default content")
}

// TestBlockWithDefinition tests that page can override block default content
func TestBlockWithDefinition(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{block "title" .}}Default Title{{end}}</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->{{define "title"}}Custom Title{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	// Page's definition should override the block's default
	require.Contains(t, buf.String(), "Custom Title")
	require.NotContains(t, buf.String(), "Default Title")
}

// TestMultipleBlocks tests multiple optional blocks in a layout
func TestMultipleBlocks(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>
<head>{{block "head-extra" .}}{{end}}</head>
<body>
{{block "content" .}}<p>Default content</p>{{end}}
{{block "footer" .}}<footer>Default footer</footer>{{end}}
</body>
</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->
{{define "content"}}<p>Custom content</p>{{end}}
{{define "head-extra"}}<meta name="description" content="test">{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	result := buf.String()
	// Page defines content and head-extra, so those should be custom
	require.Contains(t, result, "Custom content")
	require.Contains(t, result, `<meta name="description"`)
	// Page doesn't define footer, so default should be used
	require.Contains(t, result, "Default footer")
}

// TestNestedBlocks tests nested block definitions
func TestNestedBlocks(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>
{{block "outer" .}}
	<div>{{block "inner" .}}<span>Inner default</span>{{end}}</div>
{{end}}
</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->
{{define "inner"}}<span>Inner custom</span>{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	result := buf.String()
	// Page defines inner block, so it should be custom
	require.Contains(t, result, "Inner custom")
	require.NotContains(t, result, "Inner default")
	// Outer block is not overridden, so structure should be preserved
	require.Contains(t, result, "<div>")
}

// TestBlockWithData tests that block receives correct data context
func TestBlockWithData(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{block "greeting" .}}<p>Hello, {{.Name}}!</p>{{end}}</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	type Data struct {
		Name string
	}

	var buf strings.Builder
	err = page.Execute(&buf, Data{Name: "World"})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Hello, World!")
}

// TestEmptyBlockDefault tests block with empty default content
func TestEmptyBlockDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html><head>{{block "extra-css" .}}{{end}}</head><body>Content</body></html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	// Should render successfully with empty content for the block
	require.Contains(t, buf.String(), "<head></head>")
	require.Contains(t, buf.String(), "Content")
}

// TestBlockPriority tests that page definitions take priority over layout block defaults
func TestBlockPriority(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`{{block "value" .}}layout{{end}}`),
		},
		"pages/test1.html": &fstest.MapFile{
			// Define before layout is applied
			Data: []byte(`<!--layout:test-->{{define "value"}}page{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	page := NewHtmlTemplate("pages/test1", "pages/test1.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	// Page definition should win
	require.Equal(t, "page", strings.TrimSpace(buf.String()))
}

// TestComplexRealWorldScenario tests a realistic layout with multiple optional sections
func TestComplexRealWorldScenario(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/marketing.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
<html>
<head>
    {{block "meta" .}}<title>My Site</title>{{end}}
    <link rel="stylesheet" href="/css/main.css">
    {{block "extra-css" .}}{{end}}
</head>
<body>
    {{block "hero" .}}{{end}}
    <main>
        {{block "content" .}}<p>Welcome to my site</p>{{end}}
    </main>
    {{block "extra-scripts" .}}{{end}}
</body>
</html>`),
		},
		"pages/index.html": &fstest.MapFile{
			Data: []byte(`<!--layout:marketing-->
{{define "content"}}<h1>Home Page</h1>{{end}}`),
		},
		"pages/about.html": &fstest.MapFile{
			Data: []byte(`<!--layout:marketing-->
{{define "meta"}}<title>About Us</title><meta name="description" content="About our company">{{end}}
{{define "content"}}<h1>About Us</h1><p>We are awesome</p>{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/marketing", "layouts/marketing.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/marketing"] = layout

	// Test index page - minimal overrides
	index := NewHtmlTemplate("pages/index", "pages/index.html")
	err = index.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf1 strings.Builder
	err = index.Execute(&buf1, nil)
	require.NoError(t, err)
	result1 := buf1.String()

	require.Contains(t, result1, "<title>My Site</title>")
	require.Contains(t, result1, "<h1>Home Page</h1>")
	require.Contains(t, result1, `<link rel="stylesheet"`)

	// Test about page - more overrides
	about := NewHtmlTemplate("pages/about", "pages/about.html")
	err = about.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf2 strings.Builder
	err = about.Execute(&buf2, nil)
	require.NoError(t, err)
	result2 := buf2.String()

	require.Contains(t, result2, "<title>About Us</title>")
	require.Contains(t, result2, `<meta name="description"`)
	require.Contains(t, result2, "<h1>About Us</h1>")
	require.Contains(t, result2, "We are awesome")
}
