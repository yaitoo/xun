package csrf

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

var nop = func(c *xun.Context) error {
	c.WriteStatus(http.StatusOK)
	return nil
}

func TestNew(t *testing.T) {
	secretKey := []byte("test-secret-key-123")

	t.Run("set_cookie_when_missed", func(t *testing.T) {
		m := New(secretKey)

		ctx := createContext(httptest.NewRequest("GET", "/", nil))

		err := m(nop)(ctx)
		require.NoError(t, err)

		require.NotNil(t, ctx.Response.Header().Get("Set-Cookie"))
	})

	t.Run("skip_set_cookie_when_present", func(t *testing.T) {
		m := New(secretKey)

		ctx := createContext(httptest.NewRequest("GET", "/", nil))

		ctx.Request.AddCookie(&http.Cookie{
			Name:  DefaultCookieName,
			Value: "test",
		})

		err := m(nop)(ctx)
		require.NoError(t, err)

		require.Empty(t, ctx.Response.Header().Get("Set-Cookie"))
	})

	t.Run("verify_token", func(t *testing.T) {
		m := New(secretKey)

		// fails
		ctx := createContext(httptest.NewRequest("POST", "/", nil))
		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusTeapot, ctx.Response.StatusCode())

		// success
		ctx = createContext(httptest.NewRequest("POST", "/", nil))
		cookie := generateToken(&Options{
			SecretKey:  secretKey,
			CookieName: DefaultCookieName,
		})

		ctx.Request.AddCookie(cookie)

		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		// skip
		ctx = createContext(httptest.NewRequest("GET", "/", nil))
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

	t.Run("options", func(t *testing.T) {
		m := New(secretKey, WithCookie("test-cookie-name"))

		ctx := createContext(httptest.NewRequest("GET", "/", nil))

		err := m(nop)(ctx)
		require.NoError(t, err)

		require.NotNil(t, ctx.Response.Header().Get("Set-Cookie"))
		require.Contains(t, ctx.Response.Header().Get("Set-Cookie"), "test-cookie-name=")
	})

	t.Run("verify_js_token", func(t *testing.T) {
		m := New(secretKey, WithCookie("test_token"), WithJsToken())

		cookie := generateToken(&Options{
			SecretKey:  secretKey,
			CookieName: "test_token",
		})

		// fails on js token
		ctx := createContext(httptest.NewRequest("POST", "/", nil))
		ctx.Request.AddCookie(cookie)

		err := m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusTeapot, ctx.Response.StatusCode())

		// fails on js token
		ctx = createContext(httptest.NewRequest("POST", "/", nil))
		ctx.Request.AddCookie(cookie)
		ctx.Request.AddCookie(&http.Cookie{
			Name:  "js_test_token",
			Value: "",
		})
		err = m(nop)(ctx)
		require.ErrorIs(t, err, xun.ErrCancelled)
		require.Equal(t, http.StatusTeapot, ctx.Response.StatusCode())

		// success
		ctx = createContext(httptest.NewRequest("POST", "/", nil))

		ctx.Request.AddCookie(cookie)
		ctx.Request.AddCookie(&http.Cookie{
			Name:  "js_test_token",
			Value: cookie.Value,
		})

		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())

		// skip
		ctx = createContext(httptest.NewRequest("GET", "/", nil))
		err = m(nop)(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	})

}

func TestHandleFunc(t *testing.T) {

	fn := HandleFunc([]byte("secret"), WithCookie("test_token"))

	t.Run("load", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := &xun.Context{
			Request:  httptest.NewRequest("GET", "/csrf.js", nil),
			Response: xun.NewResponseWriter(w),
		}

		err := fn(ctx)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"test_token"`)
	})

	t.Run("etag", func(t *testing.T) {
		f, _ := fsys.Open("csrf.js") // nolint: errcheck
		defer f.Close()

		buf, _ := io.ReadAll(f) // nolint: errcheck

		p, _ := template.New("token").Parse(string(buf)) // nolint: errcheck

		var processed bytes.Buffer
		// nolint: errcheck
		p.Execute(&processed, &Options{
			SecretKey:  []byte("secret"),
			CookieName: "test_token",
		})

		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write(processed.Bytes())

		etag := base64.URLEncoding.EncodeToString(mac.Sum(nil))

		w := httptest.NewRecorder()
		ctx := &xun.Context{
			Request:  httptest.NewRequest("GET", "/csrf.js", nil),
			Response: xun.NewResponseWriter(w),
		}

		ctx.Request.Header.Set("If-None-Match", etag)

		err := fn(ctx)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotModified, w.Code)

	})

}

func createContext(r *http.Request) *xun.Context {
	return &xun.Context{
		Request:  r,
		Response: xun.NewResponseWriter(httptest.NewRecorder()),
	}
}
