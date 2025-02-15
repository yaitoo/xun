package acl

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func createContext(w http.ResponseWriter) *xun.Context {
	if w == nil {
		w = httptest.NewRecorder()
	}
	return &xun.Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: xun.NewResponseWriter(w),
	}
}

var nop = func(c *xun.Context) error {
	c.WriteStatus(http.StatusOK)
	return nil
}

func TestHosts(t *testing.T) {
	t.Run("allow", func(t *testing.T) {
		m := New(AllowHosts("abc.com"))

		ctx := createContext(nil)
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)

		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("host_redirect", func(t *testing.T) {
		m := New(AllowHosts("abc.com"), WithHostRedirect("http://127.0.0.2", http.StatusFound))
		w := httptest.NewRecorder()
		ctx := createContext(w)
		ctx.App = xun.New()
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)

		require.Equal(t, http.StatusFound, w.Code)
		require.Equal(t, "http://127.0.0.2", w.Header().Get("Location"))
	})

	t.Run("redirect_with_invalid_url", func(t *testing.T) {
		m := New(AllowHosts("abc.com"), WithHostRedirect("", http.StatusFound))

		ctx := createContext(nil)
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)

		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("redirect_with_invalid_code", func(t *testing.T) {
		m := New(AllowHosts("abc.com"), WithHostRedirect("http://127.0.0.1", 0))

		ctx := createContext(nil)
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)

		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})
}

func TestIPNets(t *testing.T) {

	t.Run("allow_deny", func(t *testing.T) {
		m := New(AllowIPNets("172.0.0.1"), DenyIPNets("172.0.0.1", "172.0.0.2"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.2:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("only_allow", func(t *testing.T) {
		m := New(AllowIPNets("[2001:db8:85a3:0:0:8a2e:370]:1"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "[2001:db8:85a3:0:0:8a2e:370:1]:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "[2001:db8:85a3:0:0:8a2e:370:2]:2222"
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

	t.Run("only_deny", func(t *testing.T) {
		m := New(DenyIPNets("172.0.0.1", "172.0.0.2"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.2:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())

	})

	t.Run("allow_any", func(t *testing.T) {
		m := New(AllowIPNets("*"), DenyIPNets("172.0.0.1", "192.0.0.2"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "192.0.0.2:2222"
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

	t.Run("deny_any", func(t *testing.T) {
		m := New(AllowIPNets("172.0.0.1"), DenyIPNets("*"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.2:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})
}

func TestCountries(t *testing.T) {

	lookup := func(ip string) string {
		if strings.HasPrefix(ip, "172.") {
			return "cn"
		}

		return "us"
	}

	t.Run("allow_deny", func(t *testing.T) {
		m := New(WithLookupFunc(lookup),
			AllowCountries("cn"),
			DenyCountries("us"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "192.0.0.1:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})

	t.Run("only_allow", func(t *testing.T) {
		m := New(WithLookupFunc(lookup),
			AllowCountries("cn"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "192.0.0.1:2222"
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

	t.Run("only_deny", func(t *testing.T) {
		m := New(WithLookupFunc(lookup),
			DenyCountries("cn", "us"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.2:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())

	})

	t.Run("allow_any", func(t *testing.T) {
		m := New(WithLookupFunc(lookup),
			AllowCountries("*"), DenyCountries("cn", "us"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "192.0.0.2:2222"
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

	t.Run("deny_any", func(t *testing.T) {
		m := New(WithLookupFunc(lookup),
			AllowCountries("cn"), DenyCountries("*"))

		ctx := createContext(nil)
		ctx.Request.RemoteAddr = "172.0.0.1:1111"
		err := m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		ctx = createContext(nil)
		ctx.Request.RemoteAddr = "192.0.0.2:2222"
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusForbidden, ctx.Response.StatusCode())
	})
}
