package autossl

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// AutoSSL is a wrapper around autocert.Manager that provides automatic
// management of SSL/TLS certificates. It embeds autocert.Manager to
// inherit its methods and functionalities, allowing for seamless
// integration and usage of automatic certificate management in your
// application.
type AutoSSL struct {
	*autocert.Manager
}

// New creates a new AutoSSL instance with the provided options.
// It initializes an autocert.Manager with the AcceptTOS prompt.
// If no cache is provided in the options, it defaults to using a directory cache.
//
// Parameters:
//
//	opts - A variadic list of Option functions to configure the autocert.Manager.
//
// Returns:
//
//	A pointer to an AutoSSL instance with the configured autocert.Manager.
func New(opts ...Option) *AutoSSL {
	cm := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
	}

	for _, opt := range opts {
		opt(cm)
	}

	if cm.Cache == nil {
		cm.Cache = autocert.DirCache(".")
	}

	return &AutoSSL{
		Manager: cm,
	}
}

// Configure sets up the HTTP and HTTPS servers with the necessary handlers and TLS configurations.
// It modifies the HTTP server to use the AutoSSL manager's HTTP handler and ensures the HTTPS server
// has a TLS configuration with at least TLS 1.2. It also sets the GetCertificate function for the
// HTTPS server's TLS configuration to use the AutoSSL manager's GetCertificate method.
//
// Parameters:
//   - httpSrv: A pointer to the HTTP server to be configured.
//   - httpsSrv: A pointer to the HTTPS server to be configured.
func (autossl *AutoSSL) Configure(httpSrv *http.Server, httpsSrv *http.Server) {
	httpSrv.Handler = autossl.Manager.HTTPHandler(httpSrv.Handler)

	if httpsSrv.TLSConfig == nil {
		httpsSrv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: 0,
		}
	}

	httpsSrv.TLSConfig.GetCertificate = autossl.Manager.GetCertificate
}
