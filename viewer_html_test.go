package xun

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidHtmlTemplate(t *testing.T) {

	type Data struct {
		Name string
	}

	l, err := template.New("invalid").Parse(`<p>Hello, {{.Name}}!</p><p>Age: {{.Age}}</p>`)
	require.NoError(t, err)

	v := &HtmlViewer{
		template: &HtmlTemplate{
			template: l,
		},
	}

	err = v.Render(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), Data{})
	require.NotNil(t, err)
}
