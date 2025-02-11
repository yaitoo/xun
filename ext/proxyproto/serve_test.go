package proxyproto

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListenAndServe(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) // nolint: errcheck
	}))

	srv := &http.Server{ // skipcq: GO-S2112
		Addr:    strings.TrimPrefix(s.URL, "http://"),
		Handler: s.Config.Handler,
	}

	defer srv.Close()

	t.Run("fail_to_listen", func(t *testing.T) {
		err := ListenAndServe(srv)
		require.NotNil(t, err)
	})

	s.Close()

	t.Run("listen", func(t *testing.T) {
		go ListenAndServe(srv) // nolint: errcheck

		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get(s.URL)
		require.NoError(t, err)
		defer resp.Body.Close() // skipcp: GO-S2307
		require.Equal(t, http.StatusOK, resp.StatusCode)

	})

	srv.Shutdown(context.TODO()) // nolint: errcheck

}

func TestListenAndServeTLS(t *testing.T) {

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // skip: GSC-G402,GO-S1020
	client := http.Client{
		Transport: tr,
	}

	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) // nolint: errcheck
	}))

	time.Sleep(100 * time.Millisecond)

	srv := &http.Server{ // skipcq: GO-S2112
		Addr: strings.TrimPrefix(s.URL, "https://"),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK")) // nolint: errcheck
		}),
		TLSConfig: s.Config.TLSConfig,
	}

	srv.TLSConfig.Certificates = s.TLS.Certificates
	defer srv.Close()

	t.Run("fail_to_listen", func(t *testing.T) {
		err := ListenAndServeTLS(srv, "", "")
		require.NotNil(t, err)
	})

	s.Close()

	t.Run("listen", func(t *testing.T) {
		go ListenAndServeTLS(srv, "", "") // nolint: errcheck

		time.Sleep(100 * time.Millisecond)

		resp, err := client.Get(s.URL)
		require.NoError(t, err)
		defer resp.Body.Close() // skipcq: GO-S2307
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	srv.Shutdown(context.TODO()) // nolint: errcheck
}
