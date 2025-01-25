package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileViewer(t *testing.T) {
	now := time.Now()
	fsys := &fstest.MapFS{
		"public/index.html": {Data: []byte(`<!DOCTYPE html><html><body>Hello, world!</body></html>`), ModTime: time.Now()},
	}

	// https://github.com/yaitoo/xun/issues/32
	t.Run("etag_should_work_without_mod_time", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/index.html", true)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		require.Equal(t, "*/*", v.MimeType().String())
		err := v.Render(w, r, nil)

		require.NoError(t, err)
		etag := w.Header().Get("ETag")

		require.Equal(t, v.etag, etag)
		require.Equal(t, http.StatusOK, w.Code)

		r = httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("If-None-Match", etag)

		w = httptest.NewRecorder()

		err = v.Render(w, r, nil)
		require.NoError(t, err)

		require.NotEmpty(t, etag)
		require.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("last_modified_should_work", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/index.html", false)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		err := v.Render(w, r, nil)

		require.NoError(t, err)
		lastModified := w.Header().Get("Last-Modified")
		require.NotEmpty(t, lastModified)

		modTime := now.UTC().Format(http.TimeFormat)
		require.Equal(t, modTime, lastModified)
		require.Equal(t, http.StatusOK, w.Code)

		r = httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("If-Modified-Since", modTime)

		w = httptest.NewRecorder()

		err = v.Render(w, r, nil)
		require.NoError(t, err)

		lastModified = w.Header().Get("Last-Modified")
		require.NotEmpty(t, lastModified)

		require.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("file_not_found_should_work", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/notfound.html", false)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		err := v.Render(w, r, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, w.Code)

		v = NewFileViewer(fsys, "public/notfound.html", true)
		r = httptest.NewRequest(http.MethodGet, "/", nil)
		w = httptest.NewRecorder()

		err = v.Render(w, r, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, w.Code)

	})

}
