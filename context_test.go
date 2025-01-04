package xun

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextRequestReferer(t *testing.T) {
	tests := []struct {
		name     string
		referer  string
		expected string
	}{
		{
			name:     "normal",
			referer:  "/home",
			expected: "/home",
		},
		{
			name:     "empty",
			referer:  "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := &Context{
				app: &App{},
				req: httptest.NewRequest(http.MethodGet, "/", nil),
			}

			ctx.req.Header.Set("Referer", test.referer)

			require.Equal(t, test.expected, ctx.RequestReferer())
		})
	}
}

func TestContextVars(t *testing.T) {

	srv := httptest.NewServer(http.DefaultServeMux)
	defer srv.Close()

	app := New()

	app.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			c.Set("var", "middleware")
			return next(c)
		}
	})

	app.Get("/vars", func(c *Context) error {
		return c.View(c.Get("var"))
	})

	go app.Start()
	defer app.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/vars", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var v string
	err = json.NewDecoder(resp.Body).Decode(&v)
	require.NoError(t, err)
	require.Equal(t, "middleware", v)

}
