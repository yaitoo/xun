package cookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestSet(t *testing.T) {
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
}

func TestGet(t *testing.T) {
	ctx := &xun.Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: httptest.NewRecorder(),
	}
	c := http.Cookie{Name: "test", Value: "dmFsdWU="} // base64 encoded "value"
	ctx.Request.Header.Set("Cookie", c.String())

	value, err := Get(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, "value", value)
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

func TestGetSigned(t *testing.T) {
	ts := time.Now()
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("test"))
	mac.Write([]byte(ts.Format(time.RFC3339)))
	mac.Write([]byte("value"))

	signature := mac.Sum(nil)

	value := base64.URLEncoding.EncodeToString([]byte(string(signature) + ts.Format(time.RFC3339) + "value"))

	cookie := http.Cookie{Name: "test", Value: value}
	ctx := &xun.Context{
		Request: httptest.NewRequest(http.MethodGet, "/", nil),
	}

	ctx.Request.AddCookie(&cookie)

	v, tv, err := GetSigned(ctx, "test", []byte("secret"))
	require.NoError(t, err)
	require.Equal(t, "value", v)
	require.Equal(t, ts.Format(time.RFC3339), tv.Format(time.RFC3339))

}

func TestSetSigned(t *testing.T) {
	ctx := &xun.Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: httptest.NewRecorder(),
	}
	c := http.Cookie{Name: "test", Value: "value"}
	ts, err := SetSigned(ctx, c, []byte("secret"))
	require.NoError(t, err)

	result := ctx.Response.Header().Get("Set-Cookie")
	require.NoError(t, err)

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("test"))
	mac.Write([]byte(ts.Format(time.RFC3339)))
	mac.Write([]byte("value"))

	signature := mac.Sum(nil)

	value := base64.URLEncoding.EncodeToString([]byte(string(signature) + ts.Format(time.RFC3339) + "value"))

	require.Equal(t, "test="+value, result)
}

func TestGetEncrypted(t *testing.T) {

}
