package csrf

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yaitoo/xun"
)

func TestCSRFMiddleware(t *testing.T) {
	secretKey := []byte("test-secret-key-123")

	t.Run("options configuration", func(t *testing.T) {
		mw := New(secretKey, WithCookie("custom_csrf"), WithExpire(100*time.Second))
		assert.NotNil(t, mw)
	})

	t.Run("token generation and validation", func(t *testing.T) {
		opts := &Options{
			SecretKey:  secretKey,
			CookieName: "csrf_token",
			MaxAge:     3600,
		}

		// Generate token
		expires := time.Now().Add(time.Hour)
		token := generateToken(opts, expires)
		assert.NotEmpty(t, token)

		// Verify token parts
		parts := strings.Split(token, ".")
		assert.Equal(t, 3, len(parts))

		// Validate token
		cookie := &http.Cookie{
			Name:  opts.CookieName,
			Value: token,
		}
		assert.True(t, verifyToken(cookie, secretKey))
	})

	t.Run("http methods handling", func(t *testing.T) {
		mw := New(secretKey)
		handler := func(c *xun.Context) error {
			return nil
		}

		methods := []string{"GET", "HEAD", "OPTIONS", "POST", "PUT", "DELETE"}
		for _, method := range methods {
			t.Run(method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/test", nil)
				w := httptest.NewRecorder()
				c := &xun.Context{
					Request:  req,
					Response: xun.NewResponseWriter(w),
				}

				err := mw(handler)(c)

				if method == "GET" || method == "HEAD" || method == "OPTIONS" {
					assert.NoError(t, err)
					assert.Equal(t, http.StatusOK, w.Code)
					cookies := w.Result().Cookies()
					assert.Equal(t, 1, len(cookies))
				} else {
					assert.Equal(t, xun.ErrCancelled, err)
					assert.Equal(t, http.StatusTeapot, w.Code)
				}
			})
		}
	})

	t.Run("custom expire function", func(t *testing.T) {
		customDuration := 15 * time.Minute
		mw := New(secretKey, func(o *Options) {
			o.ExpireFunc = func(c *xun.Context) (bool, time.Duration) {
				return true, customDuration
			}
		})

		handler := func(c *xun.Context) error {
			return nil
		}

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		c := &xun.Context{
			Request:  req,
			Response: xun.NewResponseWriter(w),
		}

		err := mw(handler)(c)
		assert.NoError(t, err)

		cookies := w.Result().Cookies()
		assert.Equal(t, 1, len(cookies))
		assert.Equal(t, int(customDuration.Seconds()), cookies[0].MaxAge)
	})

	t.Run("token validation edge cases", func(t *testing.T) {
		tests := []struct {
			name  string
			token *http.Cookie
			want  bool
		}{
			{
				name:  "nil cookie",
				token: nil,
				want:  false,
			},
			{
				name: "malformed token",
				token: &http.Cookie{
					Name:  "csrf_token",
					Value: "invalid",
				},
				want: false,
			},
			{
				name: "invalid base64",
				token: &http.Cookie{
					Name:  "csrf_token",
					Value: "invalid.invalid.invalid",
				},
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := verifyToken(tt.token, secretKey)
				assert.Equal(t, tt.want, got)
			})
		}
	})
}
