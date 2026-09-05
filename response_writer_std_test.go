package xun

import (
	"bufio"
	"net"
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

// hijackerResponseWriter is a minimal http.ResponseWriter + http.Hijacker
// used to verify that *stdResponseWriter.Hijack delegates to the wrapped
// writer when the wrapped writer itself implements http.Hijacker.
type hijackerResponseWriter struct {
	http.ResponseWriter
	conn net.Conn
	buf  *bufio.ReadWriter
}

func (h *hijackerResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, h.buf, nil
}

func TestStdResponseWriter_Hijack_Success(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	hj := &hijackerResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		conn:           serverConn,
		buf:            bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
	}

	std := &stdResponseWriter{ResponseWriter: hj}

	conn, buf, err := std.Hijack()
	require.NoError(t, err)
	require.Equal(t, serverConn, conn)
	require.Same(t, hj.buf, buf)
	require.True(t, std.hijacked)
}

func TestStdResponseWriter_Hijack_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	std := &stdResponseWriter{ResponseWriter: rec}

	_, _, err := std.Hijack()
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not implement Hijacker")
}

func TestResponseWriter_ImplementsFlusherAndHijacker(t *testing.T) {
	rw := NewResponseWriter(httptest.NewRecorder())
	require.Implements(t, (*http.Flusher)(nil), rw)
	require.Implements(t, (*http.Hijacker)(nil), rw)
}

func TestResponseWriter_FlushSucceedsOnRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)
	// Should not panic; httptest.ResponseRecorder implements http.Flusher.
	rw.Flush()
}
