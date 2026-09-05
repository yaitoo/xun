package xun

import (
	"bufio"
	"errors"
	"io"
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
	require.False(t, std.hijacked, "hijacked flag must not be set when type assertion fails")
}

// errorHijacker is an http.ResponseWriter + http.Hijacker that returns a
// fixed error from Hijack. It verifies that *stdResponseWriter.Hijack does
// not set the hijacked flag when the underlying Hijack fails — the wrapper
// must keep working through the encoder as if Hijack had not been called.
type errorHijacker struct {
	http.ResponseWriter
	err error
}

func (h *errorHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, h.err
}

func TestStdResponseWriter_Hijack_UnderlyingErrorDoesNotSetFlag(t *testing.T) {
	hijackErr := errors.New("simulated underlying hijack failure")
	hj := &errorHijacker{
		ResponseWriter: httptest.NewRecorder(),
		err:            hijackErr,
	}
	std := &stdResponseWriter{ResponseWriter: hj}

	_, _, err := std.Hijack()
	require.ErrorIs(t, err, hijackErr)
	require.False(t, std.hijacked, "hijacked flag must NOT be set when underlying Hijack returns an error")
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

// After a successful Hijack, the framework error-to-status path (mux closure
// writes X-Log-Id + 500 + body) must NOT reach the caller-owned conn. We
// verify by hijacking, then driving the wrapper as the framework would when
// the handler returns an error: WriteHeader(500), Write("body"). None of
// these may hit the underlying writer.
func TestStdResponseWriter_PostHijackWritesAreNoOps(t *testing.T) {
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

	std := &stdResponseWriter{ResponseWriter: hj}
	_, _, err := std.Hijack()
	require.NoError(t, err)
	require.True(t, std.hijacked)

	// Simulate framework error-to-status path. None of these may reach the
	// underlying writer or the conn.
	std.WriteHeader(http.StatusInternalServerError)
	n, err := std.Write([]byte("framework error body"))
	require.NoError(t, err)
	require.Equal(t, 0, n)
	std.Flush()

	// StatusCode should still report OK because nothing was actually written.
	require.Equal(t, http.StatusOK, std.StatusCode())
}
