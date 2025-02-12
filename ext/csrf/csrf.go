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
				c.WriteStatus(http.StatusForbidden)
				c.Response.Write([]byte("INVALID_CSRF_TOKEN")) // nolint: errcheck
				return xun.ErrCancelled
			}

			setTokenCookie(c, o)

			return next(c)
		}
	}
}

func generateToken(o *Options) string {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return ""
	}

	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(randomBytes)
	signature := mac.Sum(nil)

	token := fmt.Sprintf(
		"%s.%s",
		base64.URLEncoding.EncodeToString(randomBytes),
		base64.URLEncoding.EncodeToString(signature),
	)

	return token
}

func VerifyToken(cookieToken *http.Cookie, secretKey []byte) bool {
	if cookieToken == nil {
		return false
	}
	parts := strings.Split(cookieToken.Value, ".")
	if len(parts) != 2 {
		return false
	}

	randomBytes, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write(randomBytes)
	expectedSignature := mac.Sum(nil)

	return hmac.Equal(
		expectedSignature,
		[]byte(parts[1]),
	)
}

func setTokenCookie(c *xun.Context, o *Options) {
	token := generateToken(o)

	maxAge := o.MaxAge

	if o.ExpireFunc != nil {
		ok, d := o.ExpireFunc(c)
		if ok {
			maxAge = int(d / time.Second)
		}
	}

	http.SetCookie(c.Response, &http.Cookie{
		Name:     o.CookieName,
		Value:    token,
		MaxAge:   maxAge,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}
