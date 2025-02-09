package cookie

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestCookie(t *testing.T) {

	t.Run("set", func(t *testing.T) {
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}
		c := http.Cookie{Name: "test", Value: "value"}

		err := Set(ctx, c)
		require.NoError(t, err)

		result := ctx.Response.Header().Get("Set-Cookie")
		require.NoError(t, err)
		require.Equal(t, "test=dmFsdWU=", result) // base64 encoded "value"
	})

	t.Run("get", func(t *testing.T) {
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}
		c := http.Cookie{Name: "test", Value: "dmFsdWU="} // base64 encoded "value"
		ctx.Request.Header.Set("Cookie", c.String())

		value, err := Get(ctx, "test")
		require.NoError(t, err)
		require.Equal(t, "value", value)
	})

}

func TestDelete(t *testing.T) {
	ctx := &xun.Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: xun.NewResponseWriter(httptest.NewRecorder()),
	}
	c := http.Cookie{Name: "test", Value: "dmFsdWU="} // base64 encoded "value"
	Delete(ctx, c)

	result := ctx.Response.Header().Get("Set-Cookie")
	require.Equal(t, "test=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0", result)
}

func TestSignedCookie(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		cookie := http.Cookie{Name: "test", Value: "value"}
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		ts, err := SetSigned(ctx, cookie, []byte("secret"))
		require.NoError(t, err)

		_, signedValue := signValue([]byte("secret"), "test", "value", ts)

		expectedValue := base64.URLEncoding.EncodeToString([]byte(signedValue))

		actualValue := ctx.Response.Header().Get("Set-Cookie")
		require.Equal(t, "test="+expectedValue, actualValue)
	})

	t.Run("get", func(t *testing.T) {
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		ts := time.Now()

		_, signedValue := signValue([]byte("secret"), "test", "value", ts)

		value := base64.URLEncoding.EncodeToString([]byte(signedValue))

		c := http.Cookie{Name: "test", Value: value}
		ctx.Request.Header.Set("Cookie", c.String())

		actualValue, actualTs, err := GetSigned(ctx, "test", []byte("secret"))
		require.NoError(t, err)
		require.Equal(t, "value", actualValue)
		require.Equal(t, ts.UTC().Format(time.RFC3339), actualTs.UTC().Format(time.RFC3339))
	})

}

func TestInvalidCookie(t *testing.T) {
	t.Run("too_long_value", func(t *testing.T) {
		ctx := &xun.Context{
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		err := Set(ctx, http.Cookie{
			Name:  "test",
			Value: strings.Repeat("a", 5000),
		})

		require.ErrorIs(t, err, ErrValueTooLong)
	})

	t.Run("invalid_base64", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		ctx.Request.AddCookie(&http.Cookie{
			Name:  "test",
			Value: "aGV sbG8",
		})

		_, err := Get(ctx, "test")

		require.ErrorIs(t, err, ErrInvalidValue)
	})

	t.Run("not_exists", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		_, err := Get(ctx, "test")

		require.ErrorIs(t, err, http.ErrNoCookie)
	})

}

func TestInvalidSigned(t *testing.T) {
	t.Run("too_long_value", func(t *testing.T) {
		ctx := &xun.Context{
			Response: xun.NewResponseWriter(httptest.NewRecorder()),
		}

		_, err := SetSigned(ctx, http.Cookie{
			Name:  "test",
			Value: strings.Repeat("a", 5000),
		}, []byte("secret"))

		require.ErrorIs(t, err, ErrValueTooLong)
	})

	t.Run("invalid_base64", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		ctx.Request.AddCookie(&http.Cookie{
			Name:  "test",
			Value: "aGV sbG8",
		})

		_, _, err := GetSigned(ctx, "test", []byte("secret"))

		require.ErrorIs(t, err, ErrInvalidValue)
	})

	t.Run("not_exists", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		_, _, err := GetSigned(ctx, "test", []byte("secret"))

		require.ErrorIs(t, err, http.ErrNoCookie)
	})

	t.Run("too_less_value", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		ctx.Request.AddCookie(&http.Cookie{
			Name:  "test",
			Value: base64.URLEncoding.EncodeToString([]byte("aaa")),
		})

		_, _, err := GetSigned(ctx, "test", []byte("secret"))

		require.ErrorIs(t, err, ErrInvalidValue)
	})

	t.Run("invalid_timestamp", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		_, signedValue := signValue([]byte("secret"), "test", "value", time.Now())

		invalidValue := signedValue[:sha256.Size] + strings.Repeat("a", 20) + signedValue[sha256.Size+20:]

		ctx.Request.AddCookie(&http.Cookie{
			Name:  "test",
			Value: base64.URLEncoding.EncodeToString([]byte(invalidValue)),
		})

		_, _, err := GetSigned(ctx, "test", []byte("secret"))

		require.ErrorIs(t, err, ErrInvalidValue)
	})

	t.Run("invalid_signature", func(t *testing.T) {
		ctx := &xun.Context{
			Request: httptest.NewRequest(http.MethodGet, "/", nil),
		}

		_, signedValue := signValue([]byte("secret"), "test", "value", time.Now())

		invalidValue := strings.Repeat("a", sha256.Size) + signedValue[sha256.Size:]

		ctx.Request.AddCookie(&http.Cookie{
			Name:  "test",
			Value: base64.URLEncoding.EncodeToString([]byte(invalidValue)),
		})

		_, _, err := GetSigned(ctx, "test", []byte("secret"))

		require.ErrorIs(t, err, ErrInvalidValue)
	})

}
