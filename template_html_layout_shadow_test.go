package xun

import (
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestLayoutRootNotSkipped verifies the fix for the code review finding:
// If a page defines a template with the same name as the layout root,
// the layout root must still be added to ensure Execute() works.
//
// This test would fail before the fix where `|| ltName == layoutName`
// was added to the Lookup guard condition.
func TestLayoutRootNotSkipped(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{block "content" .}}<p>Layout default</p>{{end}}</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			// This page maliciously or accidentally defines a template
			// with the same name as the layout itself
			Data: []byte(`<!--layout:test-->
{{define "layouts/test"}}<p>Page shadowing layout name</p>{{end}}
{{define "content"}}<p>Page content</p>{{end}}`),
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

	// Execute should NOT fail with "template not defined"
	// even though the page defined a template named "layouts/test"
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err, "Execute should work even when page shadows layout name")

	// The layout should be used (not the page's shadow definition)
	result := buf.String()
	require.Contains(t, result, "<html>")
	require.Contains(t, result, "Page content")
	require.NotContains(t, result, "Page shadowing layout name")
}

// TestLayoutRootAlwaysAdded ensures that even if the page template set
// already contains a template with layoutName, the actual layout template
// is still added and used for execution.
func TestLayoutRootAlwaysAdded(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/main.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{block "body" .}}Default body{{end}}</body></html>`),
		},
		"pages/index.html": &fstest.MapFile{
			Data: []byte(`<!--layout:main-->{{define "body"}}<h1>Index</h1>{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/main", "layouts/main.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/main"] = layout

	page := NewHtmlTemplate("pages/index", "pages/index.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)

	result := buf.String()
	require.Contains(t, result, "<!DOCTYPE html>")
	require.Contains(t, result, "<h1>Index</h1>")
}
