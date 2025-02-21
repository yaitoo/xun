package acl

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

var (
	AllIPv4 = ParseIPNet("0.0.0.0/0")
	AllIPv6 = ParseIPNet("::/0")
)

// Options represents the configuration options for the ACL middleware.
// It includes various settings such as allowed hosts, IP networks, countries,
// and a lookup function for determining the country of an IP address.
type Options struct {
	AllowHosts map[string]struct{}

	HostRedirectURL        string
	HostRedirectStatusCode int
	HostWhitelist          []string

	AllowIPNets []*net.IPNet
	DenyIPNets  []*net.IPNet

	AllowCountries *CountryRule
	DenyCountries  *CountryRule

	LookupFunc func(ip string) string

	ViewerName string
	Config     string
}

type Option func(o *Options)

// AllowHosts allow the hosts
func AllowHosts(hosts ...string) Option {
	return func(o *Options) {
		for _, h := range hosts {
			o.AllowHosts[strings.ToLower(h)] = struct{}{}
		}
	}
}

// AllowIPNets allow IPNets
func AllowIPNets(nets ...string) Option {
	return func(o *Options) {
		for _, n := range nets {
			if n == "*" {
				o.AllowIPNets = append(o.AllowIPNets, AllIPv4, AllIPv6)
				continue
			}

			ipNet := ParseIPNet(n)
			if ipNet != nil {
				o.AllowIPNets = append(o.AllowIPNets, ipNet)
			}
		}
	}
}

// DenyIPNets deny IPNets
func DenyIPNets(nets ...string) Option {
	return func(o *Options) {
		for _, n := range nets {
			if n == "*" {
				o.DenyIPNets = append(o.DenyIPNets, AllIPv4, AllIPv6)
				continue
			}

			ipNet := ParseIPNet(n)
			if ipNet != nil {
				o.DenyIPNets = append(o.DenyIPNets, ipNet)
			}
		}
	}
}

// AllowCountries allow countries
func AllowCountries(countries ...string) Option {
	return func(o *Options) {
		for _, c := range countries {
			o.AllowCountries.Items[c] = struct{}{}
			if c == "*" {
				o.AllowCountries.HasAny = true
			}
		}
	}
}

// DenyCountries deny countries
func DenyCountries(countries ...string) Option {
	return func(o *Options) {
		for _, c := range countries {
			o.DenyCountries.Items[c] = struct{}{}
			if c == "*" {
				o.DenyCountries.HasAny = true
			}
		}
	}
}

// WithLookupFunc setup custom lookup function to get country by ip address
func WithLookupFunc(fn func(string) string) Option {
	return func(o *Options) {
		o.LookupFunc = fn
	}
}

// WithViewer render the viewer to current visitor when he is denied
func WithViewer(v string) Option {
	return func(o *Options) {
		o.ViewerName = v
	}
}

// WithConfig use the config file to load the options. It will watch the file and reload the options automatically.
// After WithConfig, all options will be discard, and overwritten by the config file
// Example:
//
// [allow_hosts]
// yaitoo.cn
// www.yaitoo.cn
// [allow_ipnets]
// 89.207.132.170/24
// ::1
// [deny_ipnets]
// 0.0.0.0/0
// [allow_countries]
// CN
// [deny_countries]
// *
// [host_redirect]
// url=http://abc.com
// status_code=301
func WithConfig(file string) Option {
	return func(o *Options) {
		o.Config = file
		loadOptions(file, o)
	}
}

// WithHostRedirect sets the redirect URL and status code for host redirection.
// It configures the Options to redirect requests to the specified URL with the given status code.
func WithHostRedirect(u string, code int) Option {
	return func(o *Options) {
		u, err := url.Parse(u)
		if err == nil {
			o.HostRedirectURL = u.String()
			o.HostRedirectStatusCode = code
		}
	}
}

// WithHostWhitelist sets the whitelist for AllowHosts,allowing specific paths to bypass host checking.
func WithHostWhitelist(paths ...string) Option {
	return func(o *Options) {
		o.HostWhitelist = append(o.HostWhitelist, paths...)
	}
}

// CountryRule represents a rule for managing country-based access control.
// It includes a map of country codes and a flag indicating if any country is allowed.
type CountryRule struct {
	Items  map[string]struct{}
	HasAny bool
}

func (cr *CountryRule) Has(name string) bool {
	if cr.HasAny {
		return true
	}

	_, ok := cr.Items[name]

	return ok
}

// NewOptions creates and returns a new instance of Options with default values.
// It initializes the necessary fields, including maps and slices, required for
// the ACL middleware configuration.
func NewOptions() *Options {
	return &Options{
		AllowHosts:  make(map[string]struct{}),
		AllowIPNets: make([]*net.IPNet, 0),
		DenyIPNets:  make([]*net.IPNet, 0),
		AllowCountries: &CountryRule{
			Items: make(map[string]struct{}),
		},
		DenyCountries: &CountryRule{
			Items: make(map[string]struct{}),
		},
		HostRedirectStatusCode: http.StatusFound,
	}
}
