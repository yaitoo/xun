package xun

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
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

	t.Run("hijack_inherited", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		hj := &hijackerResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			conn:           serverConn,
			buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
		}

		dw := &gzipResponseWriter{
			w: gzip.NewWriter(io.Discard),
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: hj,
			},
		}

		// gzipResponseWriter embeds *stdResponseWriter, so it must
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
		// when the pipe is full. Reads return io.EOF when both ends close.
		go func() {
			_, _ = io.Copy(io.Discard, clientConn)
		}()

		hj := &hijackerResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			conn:           serverConn,
			buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
		}

		gw := gzip.NewWriter(serverConn)
		dw := &gzipResponseWriter{
			w: gw,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: hj,
			},
		}

		_, _, err := dw.Hijack()
		require.NoError(t, err)
		require.True(t, dw.hijacked)

		// After Hijack, Close must be a no-op: it must not call gw.Close(),
		// which would write the gzip trailer onto the caller-owned stream.
		require.NotPanics(t, func() { dw.Close() })

		// gw must still be writable — gzip.Writer.Write would return an
		// error if gw.Close had been called.
		_, err = gw.Write([]byte("after-close"))
		require.NoError(t, err)
	})

	t.Run("flush_is_noop_after_hijack", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()
		go func() {
			_, _ = io.Copy(io.Discard, clientConn)
		}()

		hj := &hijackerResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			conn:           serverConn,
			buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
		}

		gw := gzip.NewWriter(serverConn)
		dw := &gzipResponseWriter{
			w: gw,
			stdResponseWriter: &stdResponseWriter{
				ResponseWriter: hj,
			},
		}

		_, _, err := dw.Hijack()
		require.NoError(t, err)

		// After Hijack, Flush must not push compressed bytes onto the
		// caller-owned stream. We verify by ensuring Flush does not panic
		// and that any underlying state remains untouched (gzip.Writer
		// itself is not closed).
		require.NotPanics(t, func() { dw.Flush() })

		_, err = gw.Write([]byte("after-flush"))
		require.NoError(t, err)
	})
}
