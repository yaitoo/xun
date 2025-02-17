package xun

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXmlViewerRenderError(t *testing.T) {
	v := &XmlViewer{}

	data := make(chan int)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	rw.Code = -1

	ctx := &Context{
		Request:  r,
		Response: NewResponseWriter(rw),
	}

	// should get raw error when xml.marshal fails, and StatusCode should be written
	err := v.Render(ctx, data)

	_, ok := err.(*xml.UnsupportedTypeError)
	require.True(t, ok)
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
