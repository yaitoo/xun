package xun

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

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
				App:     &App{},
				Request: httptest.NewRequest(http.MethodGet, "/", nil),
			}

			ctx.Request.Header.Set("Referer", test.referer)

			require.Equal(t, test.expected, ctx.RequestReferer())
		})
	}
}

func TestTempData(t *testing.T) {

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
	err = Json.NewDecoder(resp.Body).Decode(&v)
	require.NoError(t, err)
	require.Equal(t, "middleware", v)
}

func TestMixedViewers(t *testing.T) {
	fsys := fstest.MapFS{
		"views/user.html":  {Data: []byte(`user`)},
		"pages/index.html": {Data: []byte(`index`)},
		"pages/list.html":  {Data: []byte(`list`)},
		"text/robots.txt":  {Data: []byte("User-agent: *")},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := New(WithMux(mux), WithFsys(fsys))

	app.Get("/", func(c *Context) error {
		if c.Request.URL.Path == "/index" {
			return c.View(nil, "index")
		} else if c.Request.URL.Path == "/robots.txt" {
			return c.View(nil, "text/robots.txt")
		}

		c.WriteStatus(http.StatusNotFound)
		return ErrCancelled
	})

	app.Get("/{$}", func(c *Context) error {
		return c.View(nil)
	})

	app.Get("/list", func(c *Context) error {
		return c.View(map[string]any{
			"name": "list",
			"num":  2,
		})
	})

	app.Get("/view404", func(c *Context) error {
		return c.View(nil)
	}, WithViewer()) // delete default viewer

	app.Start()
	defer app.Close()

	t.Run("html_viewer_should_be_used", func(t *testing.T) {
		req, err := http.NewRequest("GET", srv.URL+"/", nil)
		req.Header.Set("Accept", "text/html")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["pages/index.html"].Data, buf)

	})

	t.Run("not_found_should_be_used", func(t *testing.T) {
		req, err := http.NewRequest("GET", srv.URL+"/404", nil)
		req.Header.Set("Accept", "text/html")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("view_not_found", func(t *testing.T) {
		req, err := http.NewRequest("GET", srv.URL+"/view404", nil)
		req.Header.Set("Accept", "text/html")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("data_bind_with_html_and_json_should_both_work", func(t *testing.T) {
		req, err := http.NewRequest("GET", srv.URL+"/list", nil)
		req.Header.Set("Accept", "text/html, */*")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["pages/list.html"].Data, buf)

		req, err = http.NewRequest("GET", srv.URL+"/list", nil)
		req.Header.Set("Accept", "application/json")
		require.NoError(t, err)
		resp, err = client.Do(req)
		require.NoError(t, err)

		buf, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		data := &struct {
			Name string `json:"name"`
			Num  int    `json:"num"`
		}{}

		err = json.Unmarshal(buf, data)

		require.NoError(t, err)

		require.Equal(t, data.Name, "list")
		require.Equal(t, data.Num, 2)
	})

	t.Run("first_viewer_should_be_used_when_accept_is_not_matched", func(t *testing.T) {
		// accept doesn't matched , first viewer(text/html) should be used
		req, err := http.NewRequest("GET", srv.URL+"/", nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["pages/index.html"].Data, buf)
	})
	t.Run("specified_viewer_should_be_used_when_accept_is_matched", func(t *testing.T) {
		// getViewer should match its viewer
		req, err := http.NewRequest("GET", srv.URL+"/index", nil)
		req.Header.Set("Accept", "text/html")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["pages/index.html"].Data, buf)
	})

	t.Run("wildcard_accept_should_be_matched_on_first_viewer", func(t *testing.T) {
		// wildcard accept should be matched on first viewer
		req, err := http.NewRequest("GET", srv.URL+"/", nil)
		req.Header.Set("Accept", "*/*")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["pages/index.html"].Data, buf)
	})

	t.Run("specified_viewer_should_be_used_when_no_any_viewer_is_matched_by_accept", func(t *testing.T) {

		req, err := http.NewRequest("GET", srv.URL+"/robots.txt", nil)
		req.Header.Set("Accept", "application/xxx")
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, fsys["text/robots.txt"].Data, buf)
	})

}

func TestDeleteHeader(t *testing.T) {
	ctx := &Context{
		Response: NewResponseWriter(httptest.NewRecorder()),
	}

	ctx.WriteHeader("test", "value")

	v := ctx.Response.Header().Get("test")
	require.Equal(t, "value", v)

	ctx.WriteHeader("test", "")

	v = ctx.Response.Header().Get("test")
	require.Empty(t, v)
}

func TestContextAccept(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []MimeType
	}{
		{
			name:     "empty",
			header:   "",
			expected: nil,
		},
		{
			name:     "single",
			header:   "text/html",
			expected: []MimeType{{Type: "text", SubType: "html"}},
		},
		{
			name:   "multiple",
			header: "text/html,application/json",
			expected: []MimeType{
				{Type: "text", SubType: "html"},
				{Type: "application", SubType: "json"},
			},
		},
		{
			name:   "with_q_value",
			header: "text/html;q=0.9,application/json;q=0.8",
			expected: []MimeType{
				{Type: "text", SubType: "html"},
				{Type: "application", SubType: "json"},
			},
		},
		{
			name:   "surrounding_whitespace",
			header: " text/html , application/json ",
			expected: []MimeType{
				{Type: "text", SubType: "html"},
				{Type: "application", SubType: "json"},
			},
		},
		{
			name:   "whitespace_around_semicolon",
			header: "text/html ; q=0.9,application/json",
			expected: []MimeType{
				{Type: "text", SubType: "html"},
				{Type: "application", SubType: "json"},
			},
		},
		{
			name:   "wildcards",
			header: "*/*,text/*",
			expected: []MimeType{
				{Type: "*", SubType: "*"},
				{Type: "text", SubType: "*"},
			},
		},
		{
			name:   "uppercase_normalized",
			header: "TEXT/HTML,Application/JSON",
			expected: []MimeType{
				{Type: "text", SubType: "html"},
				{Type: "application", SubType: "json"},
			},
		},
		{
			name:     "trailing_comma_skipped",
			header:   "text/html,",
			expected: []MimeType{{Type: "text", SubType: "html"}},
		},
		{
			name:     "leading_comma_skipped",
			header:   ",text/html",
			expected: []MimeType{{Type: "text", SubType: "html"}},
		},
		{
			name:     "only_commas",
			header:   ",,,",
			expected: nil,
		},
		{
			name:     "only_whitespace",
			header:   "   ",
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				req.Header.Set("Accept", test.header)
			}
			ctx := &Context{Request: req}

			got := ctx.Accept()

			if test.expected == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, test.expected, got)
			}
		})
	}
}

func TestContextAcceptCached(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/json")
	ctx := &Context{Request: req}

	first := ctx.Accept()

	// Mutate the header after the first call. If Accept() is properly
	// cached on the Context, the second call must return the original
	// parsed slice, not re-parse the new header.
	req.Header.Set("Accept", "application/xml,text/plain")

	second := ctx.Accept()

	require.Equal(t, first, second)
	require.Equal(t, []MimeType{
		{Type: "text", SubType: "html"},
		{Type: "application", SubType: "json"},
	}, second)
}

func TestContextAcceptEmptyHeaderCached(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept header set: Accept() should return nil and remember it.
	ctx := &Context{Request: req}

	require.Nil(t, ctx.Accept())

	req.Header.Set("Accept", "text/html")
	require.Nil(t, ctx.Accept())
}

func TestContextAcceptOnlyCommasReturnsNil(t *testing.T) {
	// Regression: pre-fix, Accept(",,,") returned a non-nil empty slice
	// because the loop pre-allocated `make([]MimeType, 0, len)`. The
	// docstring promises a nil return when there are no usable entries.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", ",,,")
	ctx := &Context{Request: req}

	require.Nil(t, ctx.Accept())
}

func TestContextAcceptLanguage(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []string
	}{
		{
			name:     "empty",
			header:   "",
			expected: nil,
		},
		{
			name:     "single",
			header:   "en-US",
			expected: []string{"en-us"},
		},
		{
			name:     "multiple",
			header:   "en-US,fr,de",
			expected: []string{"en-us", "fr", "de"},
		},
		{
			name:     "with_q_value",
			header:   "en-US;q=0.9,fr;q=0.8",
			expected: []string{"en-us", "fr"},
		},
		{
			name:     "surrounding_whitespace",
			header:   " en-US , fr ",
			expected: []string{"en-us", "fr"},
		},
		{
			name:     "trailing_comma_skipped",
			header:   "en-US,",
			expected: []string{"en-us"},
		},
		{
			name:     "leading_comma_skipped",
			header:   ",en-US",
			expected: []string{"en-us"},
		},
		{
			name:     "only_commas",
			header:   ",,,",
			expected: nil,
		},
		{
			name:     "only_whitespace",
			header:   "   ",
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				req.Header.Set("Accept-Language", test.header)
			}
			ctx := &Context{Request: req}

			got := ctx.AcceptLanguage()

			if test.expected == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, test.expected, got)
			}
		})
	}
}

func TestContextAcceptLanguageCached(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-US,fr")
	ctx := &Context{Request: req}

	first := ctx.AcceptLanguage()

	// Mutate the header after the first call. If AcceptLanguage() is
	// cached, the second call must return the original parsed slice.
	req.Header.Set("Accept-Language", "ja,zh-CN")

	second := ctx.AcceptLanguage()

	require.Equal(t, first, second)
	require.Equal(t, []string{"en-us", "fr"}, second)
}
