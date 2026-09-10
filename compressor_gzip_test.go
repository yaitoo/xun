package xun

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestGzipCompressor(t *testing.T) {
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

	app := New(WithMux(m), WithFsys(fsys), WithCompressor(&GzipCompressor{}, &DeflateCompressor{}))
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
			name:            "gzip",
			acceptEncoding:  "gzip",
			contentEncoding: "gzip",
			createReader: func(r io.Reader) io.Reader {
				gr, _ := gzip.NewReader(r)
				return gr
			},
		},
		{
			name:            "any",
			acceptEncoding:  "*",
			contentEncoding: "gzip",
			createReader: func(r io.Reader) io.Reader {
				gr, _ := gzip.NewReader(r)
				return gr
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
		{
			name:            "deflate",
			acceptEncoding:  "deflate",
			contentEncoding: "deflate",
			createReader: func(r io.Reader) io.Reader {
				return flate.NewReader(r)
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
			err = Json.NewDecoder(test.createReader(resp.Body)).Decode(&data)
			require.NoError(t, err)
			require.Equal(t, "hello", data["message"])
		})
	}

}

func TestGzipCompressor_DoubleClose(t *testing.T) {
	// Close must be idempotent. A handler that calls c.Response.Close()
	// plus the framework's defer also call Close(); without
	// idempotency the *gzip.Writer would be Put into gzipWriterPool
	// twice, letting two concurrent Gets hand the same pointer to two
	// requests and corrupt shared deflate state across them.
	//
	// The same scenario also exercises the post-Close surface: a
	// wrapper that has been closed (and therefore has rw.w == nil)
	// must not panic on subsequent Write or Flush calls.
	c := &GzipCompressor{}

	rw := httptest.NewRecorder()
	rw.Header().Set("Content-Encoding", "gzip")
	w := c.New(rw)
	gw := w.(*gzipResponseWriter)

	_, err := gw.Write([]byte("before-close"))
	require.NoError(t, err)

	w.Close()
	require.True(t, gw.closed, "first Close must set the closed flag")
	require.Nil(t, gw.w, "first Close must release the encoder pointer")

	bodyLenAfterFirst := rw.Body.Len()

	// Second Close must not panic, must not write to the recorder,
	// must not re-Put the encoder.
	require.NotPanics(t, func() { w.Close() })
	require.Equal(t, bodyLenAfterFirst, rw.Body.Len(),
		"second Close must not write additional bytes to the recorder")

	// Post-Close Write must not panic on the nil rw.w and must not
	// pull a fresh encoder out of the pool into a half-closed
	// wrapper. It returns (len(p), nil), matching the post-Hijack
	// no-op convention.
	require.NotPanics(t, func() {
		n, err := w.Write([]byte("after-close"))
		require.NoError(t, err)
		require.Equal(t, len("after-close"), n)
	})

	// Post-Close Flush must not panic on the nil rw.w.
	require.NotPanics(t, func() { w.Flush() })

	require.Equal(t, bodyLenAfterFirst, rw.Body.Len(),
		"post-Close Write/Flush must not write to the recorder")
}

func TestGzipCompressor_PoolAllocations(t *testing.T) {
	// Measure allocations over a tight Get/Close cycle. Driving the
	// measurement through httptest.NewServer + client.Do would let the
	// HTTP transport's ~60+ allocs/run dwarf the encoder signal; here
	// we exercise the compressor directly so the pooled-vs-fresh gap
	// dominates.
	//
	// Empirically: pooled is ~13 allocs/run, fresh is ~23 allocs/run.
	// Threshold 20 catches a regression that reverts to fresh
	// allocation but tolerates normal noise from
	// httptest.ResponseRecorder. Note that we cannot assert a
	// Same-pointer identity between two Get calls: sync.Pool makes
	// no LIFO guarantee and may evict the encoder between Put and
	// Get under GC pressure (notably under -race), so an
	// identity-based test is flaky. The allocation-count signal is
	// reliable across 1000 iterations because the pool's New func is
	// only invoked when the per-P cache is empty, not on every Get.

	c := &GzipCompressor{}

	// Warmup: prime the local pool so subsequent Get returns the
	// pooled encoder rather than invoking New.
	rw := httptest.NewRecorder()
	rw.Header().Set("Content-Encoding", "gzip")
	w := c.New(rw)
	w.Close()

	allocs := testing.AllocsPerRun(1000, func() {
		rw := httptest.NewRecorder()
		rw.Header().Set("Content-Encoding", "gzip")
		w := c.New(rw)
		w.Close()
	})

	require.Less(t, allocs, 20.0,
		"expected gzip encoder pooling to be active, but allocations per run were %.1f", allocs)
}

func BenchmarkGzipCompressor(b *testing.B) {
	// 16 KiB of compressible-but-not-trivial payload — typical for a
	// JSON/HTML response where the encoder's deflate state actually
	// does work, and where the per-request encoder allocation would
	// dominate without pooling.
	payload := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 380)

	m := http.NewServeMux()
	srv := httptest.NewServer(m)
	defer srv.Close()

	app := New(WithMux(m), WithCompressor(&GzipCompressor{}))
	defer app.Close()

	app.Get("/payload", func(c *Context) error {
		return c.View(payload)
	})

	go app.Start()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/payload", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")

	// Warmup: prime the pool and HTTP transport before measuring.
	for i := 0; i < 10; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
