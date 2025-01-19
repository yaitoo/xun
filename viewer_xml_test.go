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

	rw := httptest.NewRecorder()
	rw.Code = -1
	// should get raw error when xml.marshal fails, and StatusCode should be written
	err := v.Render(rw, httptest.NewRequest(http.MethodGet, "/", nil), data)

	_, ok := err.(*xml.UnsupportedTypeError)
	require.True(t, ok)
	require.Equal(t, -1, rw.Code)

	err = v.Render(rw, httptest.NewRequest(http.MethodGet, "/", nil), "")
	require.NoError(t, err)
	require.Equal(t, 200, rw.Code)

}
