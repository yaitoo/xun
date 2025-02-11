package proxyproto

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListenAndServe(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) // nolint: errcheck
	}))
	defer s.Close()

	t.Run("listen", func(t *testing.T) {
		go ListenAndServe(s.Config) // nolint: errcheck

		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get(s.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

	})

	t.Run("fail_to_listen", func(t *testing.T) {
		srv := &http.Server{
			Addr: strings.TrimPrefix(s.URL, "http://"),
		}

		defer srv.Close()

		err := ListenAndServe(srv)
		require.NotNil(t, err)
	})

}

func TestListenAndServeTLS(t *testing.T) {

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) { // skipcq: RVV-B0012
		if strings.HasPrefix(addr, "abc.com") {
			return net.Dial("tcp", strings.TrimPrefix(addr, "abc.com"))
		}
		return net.Dial("tcp", addr)
	}
	client := http.Client{
		Transport: tr,
	}

	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) // nolint: errcheck
	}))

	t.Run("listen", func(t *testing.T) {
		go ListenAndServeTLS(s.Config, "", "") // nolint: errcheck

		time.Sleep(100 * time.Millisecond)

		resp, err := client.Get(s.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

	})

	t.Run("fail_to_listen", func(t *testing.T) {

		srv := &http.Server{
			Addr: strings.TrimPrefix(s.URL, "http://"),
		}

		defer srv.Close()

		err := ListenAndServeTLS(srv, "", "")
		require.NotNil(t, err)
	})

}
