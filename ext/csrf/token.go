package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/yaitoo/xun"
)

// verifyToken checks if the given cookie token is valid.
func verifyToken(token *http.Cookie, r *http.Request, o *Options) bool {
	if token == nil {
		return false
	}

	if o.JsToken {
		jsToken, err := r.Cookie("js_" + o.CookieName)
		if err != nil {
			return false
		}

		if token.Value != jsToken.Value {
			return false
		}
	}

	parts := strings.Split(token.Value, ".")
	if len(parts) != 2 {
		return false
	}

	randomBytes, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(randomBytes)
	expected := mac.Sum(nil)

	actual, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	return hmac.Equal(expected, actual)
}

func generateToken(o *Options) *http.Cookie {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil
	}

	mac := hmac.New(sha256.New, o.SecretKey)
	mac.Write(randomBytes)
	signature := mac.Sum(nil)

	token := fmt.Sprintf(
		"%s.%s",
		base64.URLEncoding.EncodeToString(randomBytes),
		base64.URLEncoding.EncodeToString(signature),
	)

	return &http.Cookie{
		Name:     o.CookieName,
		Value:    token,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
}

func setTokenCookie(c *xun.Context, o *Options) {
	token := generateToken(o)

	if token != nil {
		http.SetCookie(c.Response, token)
	}
}
