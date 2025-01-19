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

	rw := httptest.NewRecorder()
	rw.Code = -1

	// should get raw error when json.marshal fails, and StatusCode should be written
	err := v.Render(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), data)
	require.Error(t, err)
	require.Equal(t, "chan int is unsupported type", err.Error())

	require.Equal(t, -1, rw.Code)

	err = v.Render(rw, httptest.NewRequest(http.MethodGet, "/", nil), "")
	require.NoError(t, err)
	require.Equal(t, 200, rw.Code)

}
