package xun

import (
	"bufio"
	"bytes"
	"compress/flate"
	"io"
	"net"
	"net/http"
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

	t.Run("hijack_inherited", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		hj := &hijackerResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			conn:           serverConn,
			buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
		}

		w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression) //nolint: errcheck

		dw := &deflateResponseWriter{
			w: w,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: hj,
			},
		}

		// deflateResponseWriter embeds *stdResponseWriter, so it must
		// inherit Hijack and satisfy http.Hijacker.
		var _ http.Hijacker = dw

		conn, _, err := dw.Hijack()
		require.NoError(t, err)
		require.Equal(t, serverConn, conn)
	})

	t.Run("close_skips_trailer_after_hijack", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		// Drain clientConn so the test doesn't block on net.Pipe.Close
		// when the pipe is full.
		go func() {
			_, _ = io.Copy(io.Discard, clientConn)
		}()

		hj := &hijackerResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			conn:           serverConn,
			buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
		}

		w, _ := flate.NewWriter(serverConn, flate.DefaultCompression) //nolint: errcheck
		dw := &deflateResponseWriter{
			w: w,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: hj,
			},
		}

		_, _, err := dw.Hijack()
		require.NoError(t, err)
		require.True(t, dw.hijacked)

		// After Hijack, Close must be a no-op: it must not call w.Close()
		// or w.Flush() in a way that writes onto the caller-owned stream.
		require.NotPanics(t, func() { dw.Close() })

		// w must still be writable — flate.Writer.Write would return an
		// error if w.Close had been called.
		_, err = w.Write([]byte("after-close"))
		require.NoError(t, err)
	})
}
