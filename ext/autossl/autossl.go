package autossl

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

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
			GetCertificate: cm.GetCertificate,
		},
	}

	return httpServer, httpsServer
}
