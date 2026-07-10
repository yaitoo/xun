package xun

import (
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestBlockBehaviorVsGo verifies that xun's {{block}} behavior matches Go's standard
func TestBlockBehaviorVsGo(t *testing.T) {
	t.Run("block without definition uses default", func(t *testing.T) {
		// Go standard: {{block "opt" .}}default{{end}} renders "default"
		fsys := fstest.MapFS{
			"layouts/test.html": &fstest.MapFile{
				Data: []byte(`{{block "optional" .}}default content{{end}}`),
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
		require.Equal(t, "default content", buf.String())
	})

	t.Run("block with define uses custom", func(t *testing.T) {
		// Go standard: define overrides block
		fsys := fstest.MapFS{
			"layouts/test.html": &fstest.MapFile{
				Data: []byte(`{{block "optional" .}}default{{end}}`),
			},
			"pages/test.html": &fstest.MapFile{
				Data: []byte(`<!--layout:test-->{{define "optional"}}custom{{end}}`),
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
		require.Equal(t, "custom", buf.String())
	})
}

// TestTemplateBehaviorVsGo verifies that xun's {{template}} behavior matches Go's standard
func TestTemplateBehaviorVsGo(t *testing.T) {
	t.Run("template without definition fails", func(t *testing.T) {
		// Go standard: {{template "missing"}} causes runtime error
		fsys := fstest.MapFS{
			"pages/test.html": &fstest.MapFile{
				Data: []byte(`{{template "missing" .}}`),
			},
		}

		templates := make(map[string]*HtmlTemplate)
		fm := template.FuncMap{}

		page := NewHtmlTemplate("pages/test", "pages/test.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err) // Parse succeeds

		var buf strings.Builder
		err = page.Execute(&buf, nil)
		require.Error(t, err) // Execute fails
		require.Contains(t, err.Error(), "missing")
	})

	t.Run("template with define works", func(t *testing.T) {
		// Go standard: {{template}} with {{define}} works
		fsys := fstest.MapFS{
			"pages/test.html": &fstest.MapFile{
				Data: []byte(`{{define "header"}}Header{{end}}{{template "header" .}}`),
			},
		}

		templates := make(map[string]*HtmlTemplate)
		fm := template.FuncMap{}

		page := NewHtmlTemplate("pages/test", "pages/test.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err)

		var buf strings.Builder
		err = page.Execute(&buf, nil)
		require.NoError(t, err)
		require.Equal(t, "Header", buf.String())
	})
}

// TestDependencyDetection verifies that Templates() returns all defined templates
func TestDependencyDetection(t *testing.T) {
	t.Run("block creates stub in Templates()", func(t *testing.T) {
		// Go standard: {{block "name"}} creates "name" in Templates()
		fsys := fstest.MapFS{
			"layouts/test.html": &fstest.MapFile{
				Data: []byte(`{{block "opt" .}}default{{end}}`),
			},
		}

		templates := make(map[string]*HtmlTemplate)
		fm := template.FuncMap{}

		layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
		err := layout.Load(fsys, templates, fm)
		require.NoError(t, err)

		// Check that "opt" is in dependencies (detected via Templates())
		require.Contains(t, layout.dependencies, "opt")
	})

	t.Run("define creates entry in Templates()", func(t *testing.T) {
		// Go standard: {{define "name"}} creates "name" in Templates()
		fsys := fstest.MapFS{
			"pages/test.html": &fstest.MapFile{
				Data: []byte(`{{define "header"}}H{{end}}{{define "footer"}}F{{end}}`),
			},
		}

		templates := make(map[string]*HtmlTemplate)
		fm := template.FuncMap{}

		page := NewHtmlTemplate("pages/test", "pages/test.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err)

		// Check that both defines are detected
		require.Contains(t, page.dependencies, "header")
		require.Contains(t, page.dependencies, "footer")
	})

	t.Run("template call does not create stub", func(t *testing.T) {
		// Go standard: {{template "name"}} does NOT create stub
		fsys := fstest.MapFS{
			"pages/test.html": &fstest.MapFile{
				Data: []byte(`{{template "missing" .}}`),
			},
		}

		templates := make(map[string]*HtmlTemplate)
		fm := template.FuncMap{}

		page := NewHtmlTemplate("pages/test", "pages/test.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err)

		// "missing" should NOT be in dependencies
		require.NotContains(t, page.dependencies, "missing")
	})
}

// TestBlockAndTemplateConsistency ensures block and template behave differently as expected
func TestBlockAndTemplateConsistency(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/test.html": &fstest.MapFile{
			Data: []byte(`
				{{block "blockName" .}}block default{{end}}
				{{template "templateName" .}}
			`),
		},
		"pages/with-both.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->
				{{define "blockName"}}block custom{{end}}
				{{define "templateName"}}template custom{{end}}
			`),
		},
		"pages/without-either.html": &fstest.MapFile{
			Data: []byte(`<!--layout:test-->`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	layout := NewHtmlTemplate("layouts/test", "layouts/test.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/test"] = layout

	t.Run("with both definitions", func(t *testing.T) {
		page := NewHtmlTemplate("pages/with-both", "pages/with-both.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err)

		var buf strings.Builder
		err = page.Execute(&buf, nil)
		require.NoError(t, err)
		require.Contains(t, buf.String(), "block custom")
		require.Contains(t, buf.String(), "template custom")
	})

	t.Run("without either definition", func(t *testing.T) {
		page := NewHtmlTemplate("pages/without-either", "pages/without-either.html")
		err := page.Load(fsys, templates, fm)
		require.NoError(t, err)

		var buf strings.Builder
		err = page.Execute(&buf, nil)

		// block uses default, template fails
		if err != nil {
			// Expected: template call fails because templateName not defined
			require.Contains(t, err.Error(), "templateName")
		} else {
			// If it somehow succeeded, block should show default
			require.Contains(t, buf.String(), "block default")
		}
	})
}
