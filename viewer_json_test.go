package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonViewerRenderError(t *testing.T) {
	v := &JsonViewer{}

	data := make(chan int)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	rw.Code = -1

	ctx := &Context{
		Request:  r,
		Response: NewResponseWriter(rw),
	}

	// should get raw error when json.marshal fails, and StatusCode should be written
	err := v.Render(ctx, data)
	require.Error(t, err)
	require.Equal(t, "json: unsupported type: chan int", err.Error())

	require.Equal(t, -1, rw.Code)

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	rw = httptest.NewRecorder()

	ctx = &Context{
		Request:  r,
		Response: NewResponseWriter(rw),
	}

	err = v.Render(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 200, rw.Code)

}
