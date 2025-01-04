package xun

import (
	"compress/flate"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestDeflateCompressor(t *testing.T) {
	fsys := fstest.MapFS{
		"public/skin.css": {
			Data: []byte("body { color: red; }"),
		},
		"pages/index.html": {
			Data: []byte("<html><head><title>index</title></head><body></body></html>"),
		},
	}

	m := http.NewServeMux()
	srv := httptest.NewServer(m)
	defer srv.Close()

	app := New(WithMux(m), WithFsys(fsys), WithCompressor(&DeflateCompressor{}))
	defer app.Close()

	app.Get("/json", func(c *Context) error {
		return c.View(map[string]string{"message": "hello"})
	})

	go app.Start()

	var tests = []struct {
		name            string
		acceptEncoding  string
		contentEncoding string
		createReader    func(r io.Reader) io.Reader
	}{
		{
			name:            "deflate",
			acceptEncoding:  "deflate",
			contentEncoding: "deflate",
			createReader: func(r io.Reader) io.Reader {
				return flate.NewReader(r)
			},
		},
		{
			name:            "any",
			acceptEncoding:  "*",
			contentEncoding: "deflate",
			createReader: func(r io.Reader) io.Reader {
				return flate.NewReader(r)
			},
		},
		{
			name:            "plain",
			acceptEncoding:  "",
			contentEncoding: "",
			createReader: func(r io.Reader) io.Reader {
				return r
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/skin.css", nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", test.acceptEncoding)

			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, test.contentEncoding, resp.Header.Get("Content-Encoding"))

			buf, err := io.ReadAll(test.createReader(resp.Body))
			require.NoError(t, err)
			require.Equal(t, fsys["public/skin.css"].Data, buf)

			req, err = http.NewRequest(http.MethodGet, srv.URL+"/", nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", test.acceptEncoding)

			resp, err = client.Do(req)
			require.NoError(t, err)
			require.Equal(t, test.contentEncoding, resp.Header.Get("Content-Encoding"))

			buf, err = io.ReadAll(test.createReader(resp.Body))
			require.NoError(t, err)
			require.Equal(t, fsys["pages/index.html"].Data, buf)

			req, err = http.NewRequest(http.MethodGet, srv.URL+"/json", nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", test.acceptEncoding)

			resp, err = client.Do(req)
			require.NoError(t, err)
			require.Equal(t, test.contentEncoding, resp.Header.Get("Content-Encoding"))

			data := make(map[string]string)
			err = json.NewDecoder(test.createReader(resp.Body)).Decode(&data)
			require.NoError(t, err)
			require.Equal(t, "hello", data["message"])
		})
	}

}
