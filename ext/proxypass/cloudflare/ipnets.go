package cloudflare

import "net"

type IPNets []*net.IPNet

// Contains checks if an IP address is within any of the IP networks (for IPNets compatibility)
func (l IPNets) Contains(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}

	for _, it := range l {
		if it != nil && it.Contains(ip) {
			return true
		}
	}
	return false
}

// parseIPNet parses an IP network string and returns a net.IPNet
func parseIPNet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return n
}

// toIPNets converts a slice of CIDR strings to IPNets
func toIPNets(l []string) IPNets {
	nets := make([]*net.IPNet, 0, len(l))
	for _, cidr := range l {
		if n := parseIPNet(cidr); n != nil {
			nets = append(nets, n)
		}
	}
	return nets
}
