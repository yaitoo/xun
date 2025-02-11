package reqlog

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestLogging(t *testing.T) {

	getVisitor := func(c *xun.Context) string {
		return c.Request.Header.Get("X-Visitor-Id")
	}

	getUser := func(c *xun.Context) string {
		return c.Request.Header.Get("X-User-Id")
	}

	t.Run("combined", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(Combined))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Visitor-Id", "combined-vid")
		req.Header.Set("X-User-Id", "combined-uid")
		req.Header.Set("User-Agent", "combined-agent")
		req.Header.Set("Referer", "combined-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := m(func(c *xun.Context) error {
			return nil
		})(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 200 0 \"combined-referer\" \"combined-agent\"\n"))
		require.Contains(t, l, "combined-vid combined-uid [")
	})

	t.Run("common", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(Common))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", "common-agent")
		req.Header.Set("Referer", "common-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		ctx.WriteStatus(http.StatusFound)

		err := m(func(c *xun.Context) error {
			return nil
		})(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 302 0\n"))
		require.Contains(t, l, "- - [")
	})
}
