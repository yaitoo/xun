package xun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringViewer(t *testing.T) {
	v := &StringViewer{}

	require.Equal(t, "text/plain", v.MimeType().String())

	t.Run("nil_should_be_skipped", func(t *testing.T) {

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rw := httptest.NewRecorder()
		rw.Code = -1

		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(rw),
		}

		err := v.Render(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, -1, rw.Code) // error StatusCode should not be written by StringViewer
		require.Equal(t, "text/plain; charset=utf-8", rw.Header().Get("Content-Type"))
		buf, err := io.ReadAll(rw.Body)
		require.NoError(t, err)
		require.Empty(t, buf)
	})

	t.Run("stringer_should_work", func(t *testing.T) {

		data := struct {
			Name  string
			Since int
		}{
			Name:  "xun",
			Since: 2025,
		}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rw := httptest.NewRecorder()
		rw.Code = -1

		ctx := &Context{
			Request:  r,
			Response: NewResponseWriter(rw),
		}

		err := v.Render(ctx, data)
		require.NoError(t, err)
		require.Equal(t, 200, rw.Code)
		require.Equal(t, "text/plain; charset=utf-8", rw.Header().Get("Content-Type"))
		buf, err := io.ReadAll(rw.Body)
		require.NoError(t, err)
		require.Equal(t, "{xun 2025}", string(buf))

		//

	})

}
