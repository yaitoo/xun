package acl

import (
	"net"
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

	AllowIPNets []*net.IPNet
	DenyIPNets  []*net.IPNet

	AllowCountries *CountryRule
	DenyCountries  *CountryRule

	LookupFunc func(ip string) string

	ViewerName string
	Config     string
}

type Option func(o *Options)

// AllowHost allow the hosts
func AllowHost(hosts ...string) Option {
	return func(o *Options) {
		for _, h := range hosts {
			o.AllowHosts[strings.ToLower(h)] = struct{}{}
		}
	}
}

// AllowIPNet allow IPNets
func AllowIPNet(nets ...string) Option {
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

// DenyIPNet deny IPNets
func DenyIPNet(nets ...string) Option {
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

// AllowCountry allow countries
func AllowCountry(countries ...string) Option {
	return func(o *Options) {
		for _, c := range countries {
			o.AllowCountries.Items[c] = struct{}{}
			if c == "*" {
				o.AllowCountries.HasAny = true
			}
		}
	}
}

// DenyCountry deny countries
func DenyCountry(countries ...string) Option {
	return func(o *Options) {
		for _, c := range countries {
			o.DenyCountries.Items[c] = struct{}{}
			if c == "*" {
				o.DenyCountries.HasAny = true
			}
		}
	}
}

// DenyCountry deny countries
func WithLookupFunc(fn func(string) string) Option {
	return func(o *Options) {
		o.LookupFunc = fn
	}
}

// Viewer render the viewer to current visitor when he is denied
func WithViewer(v string) Option {
	return func(o *Options) {
		o.ViewerName = v
	}
}

// WithConfig use the config file to load the options instead of program arguments. It will watch the file and reload the options automatically.
// The file should be in yaml format.
// Example:
//
//	hosts:
//		- yaitoo.cn
//		- www.yaitoo.cn
//	ipnets:
//		allow:
//			- 0.0.0.0/0
//			- 172.168.1.0/24
//			- 10.10.10.1
//		deny:
//			- ::1/128
//			-	192.168.0.1/32
//	countries:
//		allow:
//			- CN
//		deny:
//			- *

func WithConfig(file string) Option {
	return func(o *Options) {
		o.Config = file
	}
}

func WithHostRedirect(url string, code int) Option {
	return func(o *Options) {
		o.HostRedirectURL = url
		o.HostRedirectStatusCode = code
	}
}

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
	}
}
