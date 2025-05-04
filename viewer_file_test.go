package xun

import (
	"errors"
	"io/fs"
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
		"public/home.html":  {},
	}

	// https://github.com/yaitoo/xun/issues/32
	t.Run("etag_should_work_without_mod_time", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/index.html", true, "", "")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		require.Equal(t, "*/*", v.MimeType().String())
		err := v.Render(ctx, nil)

		require.NoError(t, err)
		etag := w.Header().Get("ETag")

		require.Equal(t, v.etag, etag)
		require.Equal(t, http.StatusOK, w.Code)

		r = httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("If-None-Match", etag)

		w = httptest.NewRecorder()

		ctx = &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err = v.Render(ctx, nil)
		require.NoError(t, err)

		etag = w.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("last_modified_should_work", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/index.html", false, "", "")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err := v.Render(ctx, nil)

		require.NoError(t, err)
		lastModified := w.Header().Get("Last-Modified")
		require.NotEmpty(t, lastModified)

		modTime := now.UTC().Format(http.TimeFormat)
		require.Equal(t, modTime, lastModified)
		require.Equal(t, http.StatusOK, w.Code)

		r = httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("If-Modified-Since", modTime)

		w = httptest.NewRecorder()

		ctx = &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err = v.Render(ctx, nil)
		require.NoError(t, err)

		lastModified = w.Header().Get("Last-Modified")
		require.NotEmpty(t, lastModified)

		require.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("file_not_found_should_work", func(t *testing.T) {
		v := NewFileViewer(fsys, "public/notfound.html", false, "", "")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err := v.Render(ctx, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, w.Code)

		v = NewFileViewer(fsys, "public/notfound.html", true, "", "")
		r = httptest.NewRequest(http.MethodGet, "/", nil)
		w = httptest.NewRecorder()

		ctx = &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err = v.Render(ctx, nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, w.Code)

	})

	t.Run("fails_etag_should_work", func(t *testing.T) {
		mfs := &MockFs{
			CanOpen: true,
			CanRead: false,
		}
		v := NewFileViewer(mfs, "public/home.html", true, "", "")
		require.Empty(t, v.etag)
	})

	t.Run("fails_stat_should_work", func(t *testing.T) {
		mfs := &MockFs{
			CanOpen: true,
			CanRead: false,
			CanStat: false,
		}
		v := NewFileViewer(mfs, "public/home.html", true, "", "")
		require.Empty(t, v.etag)

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(w),
		}

		err := v.Render(ctx, nil)

		require.ErrorIs(t, err, errCannotStat)
	})

}

type MockFs struct {
	fstest.MapFS
	CanOpen bool
	CanRead bool
	CanStat bool
}

func (m *MockFs) Open(name string) (fs.File, error) { // skipcq: RVV-B0012
	if m.CanOpen {
		return &MockFile{
			CanRead: m.CanRead,
			CanStat: m.CanStat,
		}, nil
	}

	return nil, errors.New("mock: can't open file")
}

type MockFile struct {
	*fstest.MapFile
	CanRead bool
	CanStat bool
}

var errCannotStat = errors.New("mock: can't stat")

func (f *MockFile) Stat() (fs.FileInfo, error) {
	if f.CanStat {
		return nil, nil
	}

	return nil, errCannotStat

}
func (f *MockFile) Read([]byte) (int, error) {
	if f.CanRead {
		return 1, nil
	}

	return 0, errors.New("mock: can't read")
}
func (*MockFile) Close() error {
	return nil
}
