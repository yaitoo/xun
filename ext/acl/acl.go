// This package provides Access Control List (ACL) middleware for the Xun framework.
// It allows for configuring allowed and denied hosts, IP networks, and countries
// based on configuration files. The middleware supports dynamic reloading of rules
// when the configuration file changes, enabling real-time updates to access rules.
// It also offers functionality for host redirection and integrates with the Xun
// framework's context to apply these rules to incoming requests.

package acl

import (
	"log"
	"net"
	"strings"
	"sync/atomic"

	"github.com/yaitoo/xun"
)

var (
	Logger = log.Default()
	v      atomic.Value
)

// New returns a new ACL middleware that applies access rules based on the provided options.
// It dynamically reloads rules if a configuration file is specified, enabling real-time updates.
func New(opts ...Option) xun.Middleware { // skipcq: GO-R1005
	options := NewOptions()

	for _, opt := range opts {
		opt(options)
	}

	v.Store(options)

	if options.Config != "" {
		go watch(options.Config, &v)
	}

	return func(next xun.HandleFunc) xun.HandleFunc {
		return func(c *xun.Context) error {
			var host = c.Request.Host

			addr, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err != nil {
				return next(c)
			}

			m := Model{
				Host: strings.ToLower(host),
				IP:   addr,
			}

			o := v.Load().(*Options)
			if len(o.AllowHosts) > 0 {
				_, allow := o.AllowHosts[m.Host]
				if !allow {
					for _, it := range o.HostWhitelist {
						if strings.EqualFold(c.Request.URL.Path, it) {
							allow = true
							break
						}
					}
				}

				if !allow {
					if o.HostRedirectStatusCode > 0 && o.HostRedirectURL != "" {
						return redirect(c, o)
					}

					return reject(c, o, m)
				}
			}

			ip := net.ParseIP(addr)
			if ip == nil {
				return next(c)
			}

			// in allow list
			if contains(ip, o.AllowIPNets) {
				return next(c)
			}

			// in deny list
			if contains(ip, o.DenyIPNets) {
				return reject(c, o, m)
			}

			if o.LookupFunc != nil {

				m.Country = o.LookupFunc(addr)

				// in allow list
				if o.AllowCountries.Has(m.Country) {
					return next(c)
				}

				// in deny list
				if o.DenyCountries.Has(m.Country) {
					return reject(c, o, m)
				}
			}

			return next(c)
		}
	}
}
