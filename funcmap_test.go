package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestFuncMap(t *testing.T) {

	fsys := fstest.MapFS{
		"pages/upper.html": &fstest.MapFile{
			Data: []byte(`{{ToUpder "upper"}}`),
		},
		"pages/lower.html": &fstest.MapFile{
			Data: []byte(`{{ToLower "Lower"}}`),
		},
		"pages/contains.html": &fstest.MapFile{
			Data: []byte(`{{Contains "hello world" "world"}}`),
		},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys),
		WithTemplateFunc("ToUpder", strings.ToUpper),
		WithTemplateFuncMap(template.FuncMap{
			"ToLower":  strings.ToLower,
			"Contains": strings.Contains,
		}))

	app.Start()
	defer app.Close()

	req, err := http.NewRequest("GET", srv.URL+"/upper", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	require.Equal(t, "UPPER", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/lower", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf, _ = io.ReadAll(resp.Body)
	require.Equal(t, "lower", string(buf))

	req, err = http.NewRequest("GET", srv.URL+"/contains", nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf, _ = io.ReadAll(resp.Body)
	require.Equal(t, "true", string(buf))
}
