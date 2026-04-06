package autotls

import (
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Option is a function type that takes a pointer to autocert.Manager as an argument.
// It is used to configure the autocert.Manager with various options.
type Option func(*autocert.Manager)

// WithCache sets the cache for the autocert.Manager.
// It takes an autocert.Cache as an argument and returns an Option
// that configures the autocert.Manager to use the provided cache.
//
// Example usage:
//
//	cache := autocert.DirCache("/path/to/cache")
//	manager := &autocert.Manager{
//	    Prompt:     autocert.AcceptTOS,
//	    Cache:      cache,
//	}
//	option := WithCache(cache)
//	option(manager)
//
// Parameters:
//   - cache: The autocert.Cache to be used by the autocert.Manager.
func WithCache(cache autocert.Cache) Option {
	return func(cm *autocert.Manager) {
		cm.Cache = cache
	}
}

// WithHosts returns an Option that sets the HostPolicy of the autocert.Manager
// to whitelist the provided hosts. This ensures that the manager will only
// respond to requests for the specified hosts.
//
// Parameters:
//
//	hosts - A variadic string parameter representing the hostnames to be whitelisted.
func WithHosts(hosts ...string) Option {
	return func(cm *autocert.Manager) {
		cm.HostPolicy = autocert.HostWhitelist(hosts...)
	}
}

func WithDirectoryURL(url string) Option {
	return func(cm *autocert.Manager) {
		if cm.Client == nil {
			cm.Client = &acme.Client{}
		}
		cm.Client.DirectoryURL = url
	}
}
