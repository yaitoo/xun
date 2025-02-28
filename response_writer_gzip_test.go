package xun

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGzipResponseWriter(t *testing.T) {
	t.Run("flush", func(t *testing.T) {

		rw := httptest.NewRecorder()

		w := gzip.NewWriter(rw)

		dw := &gzipResponseWriter{
			w: w,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: rw,
			},
		}

		_, err := w.Write([]byte("chunk1"))
		require.NoError(t, err)

		r, _ := gzip.NewReader(bytes.NewReader(rw.Body.Bytes()))

		buf, _ := io.ReadAll(r)

		require.Len(t, buf, 0)

		dw.Flush()

		r, _ = gzip.NewReader(bytes.NewReader(rw.Body.Bytes()))

		buf, _ = io.ReadAll(r)

		require.Equal(t, "chunk1", string(buf))

	})
}
