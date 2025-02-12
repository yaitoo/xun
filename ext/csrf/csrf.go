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
			if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
				_, err := c.Request.Cookie(o.CookieName)
				if err != nil { // csrf_token doesn't exists
					token, maxAge := setTokenCookie(c, o)
					if o.JsToken {
						defer func() {
							// nolint: errcheck
							c.Response.Write([]byte(
								fmt.Sprintf(`<script type="text/javascript">document.cookie="js_%s=%s;Max-Age=%v; path=/; SameSite=Lax"</script>`,
									o.CookieName, token, maxAge)))
						}()
					}
				}

				return next(c)
			}

			if !verifyToken(c.Request, o) {
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

func verifyToken(r *http.Request, o *Options) bool {
	cookieToken, err := r.Cookie(o.CookieName)
	if err != nil {
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

	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(randomBytes)
	expectedSignature := mac.Sum(nil)

	return hmac.Equal(
		expectedSignature,
		[]byte(parts[1]),
	)
}

func setTokenCookie(c *xun.Context, o *Options) (string, int) {
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
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	return token, maxAge
}
