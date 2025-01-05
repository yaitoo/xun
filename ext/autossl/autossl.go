package autossl

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// New creates and returns two HTTP servers: one for HTTP and one for HTTPS.
// The HTTP server redirects all traffic to HTTPS using the autocert.Manager's HTTPHandler.
// The HTTPS server uses the provided ServeMux and autocert.Manager for automatic TLS certificate management.
//
// Parameters:
//
//	mux - The HTTP request multiplexer to use for the HTTPS server.
//	opts - A variadic list of Option functions to configure the autocert.Manager.
//
// Returns:
//
//	*http.Server - The HTTP server configured to redirect traffic to HTTPS.
//	*http.Server - The HTTPS server configured with automatic TLS certificate management.
func New(mux *http.ServeMux, opts ...Option) (*http.Server, *http.Server) {
	cm := autocert.Manager{
		Prompt: autocert.AcceptTOS,
	}

	for _, opt := range opts {
		opt(&cm)
	}

	if cm.Cache == nil {
		cm.Cache = autocert.DirCache(".")
	}

	httpServer := &http.Server{
		Addr:    ":http",
		Handler: cm.HTTPHandler(mux),
	}

	httpsServer := &http.Server{
		Addr:    ":https",
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			MaxVersion:     0,
			GetCertificate: cm.GetCertificate,
		},
	}

	return httpServer, httpsServer
}
