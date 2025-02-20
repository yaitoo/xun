package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestETag(t *testing.T) {

	t.Run("tag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		require.True(t, !WriteIfNoneMatch(w, req))

		req.Header.Set("If-None-Match", "\"737060cd8c284d8af7ad3082f209582d\"")
		require.True(t, !WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", "\"737060cd8c284d8af7ad3082f209582d\"")
		require.True(t, WriteIfNoneMatch(w, req))
	})

	t.Run("weak_tag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		require.True(t, !WriteIfNoneMatch(w, req))

		req.Header.Set("If-None-Match", "W/\"737060cd8c284d8af7ad3082f209582d\"")
		require.True(t, !WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", "W/\"737060cd8c284d8af7ad3082f209582d\"")
		require.True(t, WriteIfNoneMatch(w, req))
	})

	t.Run("multi-etags", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		req.Header.Set("If-None-Match", `"etag1", "etag2", W/"weak-etag"`)

		w.Header().Set("ETag", `"etag1"`)
		require.True(t, WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", `"etag2"`)
		require.True(t, WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", `W/"weak-etag"`)
		require.True(t, WriteIfNoneMatch(w, req))
	})

	t.Run("any_etags", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		req.Header.Set("If-None-Match", `*`)

		w.Header().Set("ETag", `"etag1"`)
		require.True(t, WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", `"etag2"`)
		require.True(t, WriteIfNoneMatch(w, req))

		w.Header().Set("ETag", `W/"weak-etag"`)
		require.True(t, WriteIfNoneMatch(w, req))
	})

	t.Run("invalid_etags", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		req.Header.Set("If-None-Match", `W\`)

		require.False(t, WriteIfNoneMatch(w, req))

		req.Header.Set("If-None-Match", `""`)
		require.False(t, WriteIfNoneMatch(w, req))

		req.Header.Set("If-None-Match", `"etag",`)
		require.False(t, WriteIfNoneMatch(w, req))
	})
}
