package xun

import (
	"bytes"
	"compress/flate"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeflateResponseWriter(t *testing.T) {
	t.Run("flush", func(t *testing.T) {

		rw := httptest.NewRecorder()

		w, _ := flate.NewWriter(rw, flate.DefaultCompression) //nolint: errcheck because flate.DefaultCompression is a valid compression level

		dw := &deflateResponseWriter{
			w: w,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: rw,
			},
		}

		_, err := w.Write([]byte("chunk1"))
		require.NoError(t, err)

		r := flate.NewReader(bytes.NewReader(rw.Body.Bytes()))

		buf, _ := io.ReadAll(r)

		require.Len(t, buf, 0)

		dw.Flush()

		r = flate.NewReader(bytes.NewReader(rw.Body.Bytes()))

		buf, _ = io.ReadAll(r)

		require.Equal(t, "chunk1", string(buf))

	})
}
