package autossl

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/acme/autocert"
)

func TestNew(t *testing.T) {
	as := New()

	require.NotNil(t, as)
	require.NotNil(t, as.Manager)
	require.True(t, as.Prompt(""))
	require.NotNil(t, as.Manager.Cache)
}

func TestConfigure(t *testing.T) {
	as := New()
	require.NotNil(t, as)

	httpSrv := &http.Server{}  // skipcp: GO-S2112
	httpsSrv := &http.Server{} // skipcq: GSC-G402

	as.Configure(httpSrv, httpsSrv)

	require.NotNil(t, httpSrv.Handler)
	require.NotNil(t, httpsSrv.TLSConfig)

	require.Equal(t, 3*time.Second, httpSrv.ReadHeaderTimeout)
	require.Equal(t, 3*time.Second, httpsSrv.ReadHeaderTimeout)

	require.Equal(t, uint16(tls.VersionTLS12), httpsSrv.TLSConfig.MinVersion)
	require.Equal(t, uint16(0), httpsSrv.TLSConfig.MaxVersion)

	require.NotNil(t, httpsSrv.TLSConfig.GetCertificate)

	httpSrv = &http.Server{
		ReadHeaderTimeout: 1 * time.Second,
	}
	httpsSrv = &http.Server{
		ReadHeaderTimeout: 1 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS10, // skipcq: GSC-G402
			MaxVersion: tls.VersionTLS13,
		},
	}

	as.Configure(httpSrv, httpsSrv)
	require.NotNil(t, httpSrv.Handler)
	require.NotNil(t, httpsSrv.TLSConfig)

	require.Equal(t, 1*time.Second, httpSrv.ReadHeaderTimeout)
	require.Equal(t, 1*time.Second, httpsSrv.ReadHeaderTimeout)

	require.Equal(t, uint16(tls.VersionTLS10), httpsSrv.TLSConfig.MinVersion)
	require.Equal(t, uint16(tls.VersionTLS13), httpsSrv.TLSConfig.MaxVersion)

	require.NotNil(t, httpsSrv.TLSConfig.GetCertificate)
}

func TestOptions(t *testing.T) {
	as := New(WithCache(autocert.DirCache(".")), WithHosts("abc.com"))

	require.NotNil(t, as)
	require.IsType(t, autocert.DirCache("."), as.Manager.Cache)
	require.Nil(t, as.Manager.HostPolicy(context.TODO(), "abc.com"))
	require.NotNil(t, as.Manager.HostPolicy(context.TODO(), "123.com"))

}
