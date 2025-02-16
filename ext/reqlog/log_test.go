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

var nop = func(c *xun.Context) error {
	c.WriteStatus(http.StatusOK)
	return nil
}

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
		require.Contains(t, l, `"combined-vid" "combined-uid" [`)
	})

	t.Run("vcombined", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(VCombined))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "abc.com"
		req.Header.Set("Host", "abc.com")
		req.Header.Set("X-Visitor-Id", "combined-vid")
		req.Header.Set("X-User-Id", "combined-uid")
		req.Header.Set("User-Agent", "combined-agent")
		req.Header.Set("Referer", "combined-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasPrefix(l, `abc.com:80 192.0.2.1 "combined-vid" "combined-uid" [`))
		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 200 0 \"combined-referer\" \"combined-agent\"\n"))
	})

	t.Run("vcombined_custom_port", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(VCombined))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "abc.com:8080"
		req.Header.Set("Host", "abc.com")
		req.Header.Set("X-Visitor-Id", "combined-vid")
		req.Header.Set("X-User-Id", "combined-uid")
		req.Header.Set("User-Agent", "combined-agent")
		req.Header.Set("Referer", "combined-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasPrefix(l, `abc.com:8080 192.0.2.1 "combined-vid" "combined-uid" [`))
		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 200 0 \"combined-referer\" \"combined-agent\"\n"))
	})

	t.Run("vcombined_https_port", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(VCombined))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "abc.com"
		req.Header.Set("Host", "abc.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Visitor-Id", "combined-vid")
		req.Header.Set("X-User-Id", "combined-uid")
		req.Header.Set("User-Agent", "combined-agent")
		req.Header.Set("Referer", "combined-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasPrefix(l, `abc.com:443 192.0.2.1 "combined-vid" "combined-uid" [`))
		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 200 0 \"combined-referer\" \"combined-agent\"\n"))
	})

	t.Run("vcombined_empty_host", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithUser(getUser),
			WithVisitor(getVisitor),
			WithFormat(VCombined))

		req := httptest.NewRequest(http.MethodGet, "http://123.com/", nil)
		req.Host = ""
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Visitor-Id", "combined-vid")
		req.Header.Set("X-User-Id", "combined-uid")
		req.Header.Set("User-Agent", "combined-agent")
		req.Header.Set("Referer", "combined-referer")

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasPrefix(l, `123.com:443 192.0.2.1 "combined-vid" "combined-uid" [`))
		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 200 0 \"combined-referer\" \"combined-agent\"\n"))
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

		err := m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 302 0\n"))
		require.Contains(t, l, "- - [")
	})

	t.Run("skip", func(t *testing.T) {
		buf := bytes.Buffer{}

		logger := log.New(&buf, "", 0)
		m := New(WithLogger(logger),
			WithSkip(func(c *xun.Context) bool {
				return c.Request.URL.Path == "/skip"
			}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		ctx := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		ctx.WriteStatus(http.StatusFound)

		err := m(nop)(ctx)

		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodGet, "/skip", nil)
		ctx = &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		ctx.WriteStatus(http.StatusFound)

		err = m(nop)(ctx)

		require.NoError(t, err)

		l := buf.String()

		require.True(t, strings.HasSuffix(l, "] \"GET / HTTP/1.1\" 302 0 \"\" \"\"\n"))
		require.Contains(t, l, "- - [")
	})
}
