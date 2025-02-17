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
		template: NewTextTemplate(l),
	}

	ctx := &Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: NewResponseWriter(httptest.NewRecorder()),
	}

	err = v.Render(ctx, Data{})
	require.NotNil(t, err)
}
