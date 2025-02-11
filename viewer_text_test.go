package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestInvalidTextTemplate(t *testing.T) {

	type Data struct {
		Name string
	}

	l, err := template.New("invalid").Parse(`<p>Hello, {{.Name}}!</p><p>Age: {{.Age}}</p>`)
	require.NoError(t, err)

	v := &TextViewer{
		template: &TextTemplate{
			template: l,
		},
	}

	err = v.Render(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), Data{})
	require.NotNil(t, err)
}
