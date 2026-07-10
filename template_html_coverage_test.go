package xun

import (
	"html/template"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestLoadEmptyTemplate tests loading an empty template file
func TestLoadEmptyTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/empty.html": &fstest.MapFile{
			Data: []byte(``), // Empty file
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/empty", "pages/empty.html")
	err := page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Empty template should load successfully
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)
	require.Empty(t, buf.String())
}

// TestLoadInvalidTemplate tests loading a template with syntax errors
func TestLoadInvalidTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/invalid.html": &fstest.MapFile{
			Data: []byte(`<html>{{.Invalid syntax}}</html>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/invalid", "pages/invalid.html")
	err := page.Load(fsys, templates, fm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "template")
}

// TestLoadLayoutNotFound tests when layout is specified but not found in templates registry
func TestLoadLayoutNotFound(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:nonexistent--><p>Content</p>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err := page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Layout not found, but loading should succeed (just won't use layout)
	require.Equal(t, "layouts/nonexistent", page.layout)

	// Execute should work (will try to use layout that doesn't exist and fail)
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.Error(t, err) // Will fail because layout template doesn't exist
}

// TestLoadEmptyLayoutName tests when layout comment exists but name is empty
func TestLoadEmptyLayoutName(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:   --><p>Content</p>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err := page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Empty layout name should result in no layout
	require.Empty(t, page.layout)

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "<p>Content</p>")
}

// TestLoadLayoutCommentNewline tests when layout comment has newline before closing
func TestLoadLayoutCommentNewline(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/test.html": &fstest.MapFile{
			Data: []byte("<!--layout:test\n--><p>Content</p>"),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err := page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Newline should break layout parsing
	require.Empty(t, page.layout)
}

// TestLoadWithDependencies tests loading templates with define blocks
func TestLoadWithDependencies(t *testing.T) {
	fsys := fstest.MapFS{
		"components/header.html": &fstest.MapFile{
			Data: []byte(`{{define "header"}}<header>Header</header>{{end}}`),
		},
		"pages/test.html": &fstest.MapFile{
			// Using {{define}} in the same file creates dependencies
			Data: []byte(`<html>{{define "nav"}}<nav>Nav</nav>{{end}}{{template "nav" .}}</html>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	// Load component
	header := NewHtmlTemplate("components/header", "components/header.html")
	err := header.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["components/header"] = header

	// Check that component's define created a dependency
	require.Contains(t, header.dependencies, "header")

	// Load page with inline define
	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Check that inline define was detected as dependency
	require.Contains(t, page.dependencies, "nav")

	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "<nav>Nav</nav>")
}

// TestLoadWithMissingDependency tests when template references undefined template
func TestLoadWithMissingDependency(t *testing.T) {
	fsys := fstest.MapFS{
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{template "nonexistent" .}}</html>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err := page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Load succeeds, but Execute will fail
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent")
}

// TestReloadWithDependents tests reloading a template that has dependents
func TestReloadWithDependents(t *testing.T) {
	fsys := fstest.MapFS{
		"layouts/main.html": &fstest.MapFile{
			Data: []byte(`<html>{{block "content" .}}Default{{end}}</html>`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<!--layout:main-->{{define "content"}}Test{{end}}`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	// Load layout
	layout := NewHtmlTemplate("layouts/main", "layouts/main.html")
	err := layout.Load(fsys, templates, fm)
	require.NoError(t, err)
	templates["layouts/main"] = layout

	// Load page (becomes dependent of layout)
	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys, templates, fm)
	require.NoError(t, err)

	// Register the dependency relationship
	layout.dependents["pages/test"] = page

	// Reload layout should also reload page
	err = layout.Reload(fsys, templates, fm)
	require.NoError(t, err)

	// Page should still work
	var buf strings.Builder
	err = page.Execute(&buf, nil)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Test")
}

// TestReloadWithMissingDependent tests reloading when a dependent file no longer exists
func TestReloadWithMissingDependent(t *testing.T) {
	// Initial filesystem with both files
	fsys1 := fstest.MapFS{
		"components/button.html": &fstest.MapFile{
			Data: []byte(`{{define "button"}}<button>Click</button>{{end}}`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{template "button" .}}</html>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	button := NewHtmlTemplate("components/button", "components/button.html")
	err := button.Load(fsys1, templates, fm)
	require.NoError(t, err)
	templates["components/button"] = button

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys1, templates, fm)
	require.NoError(t, err)

	button.dependents["pages/test"] = page

	// New filesystem without pages/test.html
	fsys2 := fstest.MapFS{
		"components/button.html": &fstest.MapFile{
			Data: []byte(`{{define "button"}}<button>Updated</button>{{end}}`),
		},
	}

	// Reload should succeed and remove the missing dependent
	err = button.Reload(fsys2, templates, fm)
	require.NoError(t, err)

	// Dependent should be removed
	require.NotContains(t, button.dependents, "pages/test")
}

// TestReloadDependentError tests reloading when dependent has a non-NotExist error
func TestReloadDependentError(t *testing.T) {
	fsys1 := fstest.MapFS{
		"components/button.html": &fstest.MapFile{
			Data: []byte(`{{define "button"}}<button>Click</button>{{end}}`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{template "button" .}}</html>`),
		},
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	button := NewHtmlTemplate("components/button", "components/button.html")
	err := button.Load(fsys1, templates, fm)
	require.NoError(t, err)
	templates["components/button"] = button

	page := NewHtmlTemplate("pages/test", "pages/test.html")
	err = page.Load(fsys1, templates, fm)
	require.NoError(t, err)

	button.dependents["pages/test"] = page

	// New filesystem with invalid syntax in dependent
	fsys2 := fstest.MapFS{
		"components/button.html": &fstest.MapFile{
			Data: []byte(`{{define "button"}}<button>Updated</button>{{end}}`),
		},
		"pages/test.html": &fstest.MapFile{
			Data: []byte(`<html>{{.Invalid syntax}}</html>`),
		},
	}

	// Reload should fail with parse error (not NotExist error)
	err = button.Reload(fsys2, templates, fm)
	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist)
}

// TestLoadFileReadError tests error when file cannot be read
func TestLoadFileReadError(t *testing.T) {
	fsys := fstest.MapFS{
		// File doesn't exist
	}

	templates := make(map[string]*HtmlTemplate)
	fm := template.FuncMap{}

	page := NewHtmlTemplate("pages/missing", "pages/missing.html")
	err := page.Load(fsys, templates, fm)
	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrNotExist)
}
