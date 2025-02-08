package cookie

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestCookie(t *testing.T) {

	t.Run("set", func(t *testing.T) {
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: httptest.NewRecorder(),
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
			Response: httptest.NewRecorder(),
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
		Response: httptest.NewRecorder(),
	}
	c := http.Cookie{Name: "test", Value: "dmFsdWU="} // base64 encoded "value"
	Delete(ctx, c)

	result := ctx.Response.Header().Get("Set-Cookie")
	require.Equal(t, "test=; Max-Age=0", result)
}

func TestSignedCookie(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		cookie := http.Cookie{Name: "test", Value: "value"}
		ctx := &xun.Context{
			Request:  httptest.NewRequest(http.MethodGet, "/", nil),
			Response: httptest.NewRecorder(),
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
			Response: httptest.NewRecorder(),
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
