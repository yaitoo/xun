package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yaitoo/xun"
)

func New(secretKey []byte, opts ...Option) xun.Middleware {
	o := &Options{
		SecretKey:  secretKey,
		CookieName: "csrf_token",
		MaxAge:     86400, // 24hours
	}

	for _, opt := range opts {
		opt(o)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			ct, _ := c.Request.Cookie(o.CookieName) // nolint: errcheck

			if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
				if ct == nil { // csrf_token doesn't exists
					setTokenCookie(c, o)
				}

				return next(c)
			}

			if !VerifyToken(ct, o.SecretKey) {
				c.WriteStatus(http.StatusTeapot)
				return xun.ErrCancelled
			}

			setTokenCookie(c, o)

			return next(c)
		}
	}
}

func generateToken(o *Options, expires time.Time) string {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return ""
	}
	ts := []byte(expires.UTC().Format(time.RFC3339))
	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(randomBytes)
	mac.Write(ts)
	signature := mac.Sum(nil)

	token := fmt.Sprintf(
		"%s.%v.%s",
		base64.URLEncoding.EncodeToString(randomBytes),
		base64.URLEncoding.EncodeToString(ts),
		base64.URLEncoding.EncodeToString(signature),
	)

	return token
}

func VerifyToken(cookieToken *http.Cookie, secretKey []byte) bool {
	if cookieToken == nil {
		return false
	}
	parts := strings.Split(cookieToken.Value, ".")
	if len(parts) != 3 {
		return false
	}

	randomBytes, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	ts, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	expires, err := time.Parse(time.RFC3339, string(ts))
	if err != nil {
		return false
	}

	if time.Now().After(expires) {
		return false
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write(randomBytes)
	mac.Write(ts)
	expected := mac.Sum(nil)

	actual, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	return hmac.Equal(expected, actual)
}

func setTokenCookie(c *xun.Context, o *Options) {

	maxAge := o.MaxAge

	if o.ExpireFunc != nil {
		ok, d := o.ExpireFunc(c)
		if ok {
			maxAge = int(d / time.Second)
		}
	}

	expires := time.Now().Add(time.Duration(maxAge * int(time.Second)))
	token := generateToken(o, expires)

	http.SetCookie(c.Response, &http.Cookie{
		Name:     o.CookieName,
		Value:    token,
		MaxAge:   maxAge,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}
