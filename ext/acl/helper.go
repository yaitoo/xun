package acl

import (
	"errors"
	"net"
	"net/http"

	"github.com/yaitoo/xun"
)

var ErrInvalidRemoteAddr = errors.New("acl: invalid_remote_addr")

// ParseIPNet parses the given IP address and returns its IPNet.
// If the IP address does not have a netmask, the default netmask is used.
func ParseIPNet(n string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(n)
	if err != nil {
		ip := net.ParseIP(n)
		if ip != nil {
			mask := net.CIDRMask(len(ip)*8, len(ip)*8)
			ipNet = &net.IPNet{IP: ip.Mask(mask), Mask: mask}
		} else {
			return nil
		}
	}
	return ipNet
}

func redirect(c *xun.Context, o *Options) error {
	c.Redirect(o.HostRedirectURL, o.HostRedirectStatusCode)
	return xun.ErrCancelled
}

func reject(c *xun.Context, o *Options, m Model) error {
	c.WriteStatus(http.StatusForbidden)
	if o.ViewerName != "" {
		c.View(m, o.ViewerName) // nolint: errcheck
	}

	return xun.ErrCancelled
}

func contains(ip net.IP, list []*net.IPNet) bool {
	for _, ipNet := range list {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

type Model struct {
	Host    string
	IP      string
	Country string
}
