package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriterStatus(t *testing.T) {
	rw := NewResponseWriter(httptest.NewRecorder())

	require.Equal(t, http.StatusOK, rw.StatusCode())

	rw.WriteHeader(http.StatusNotFound)
	require.Equal(t, http.StatusNotFound, rw.StatusCode())

	rw.WriteHeader(http.StatusInternalServerError)
	require.Equal(t, http.StatusNotFound, rw.StatusCode())

}
