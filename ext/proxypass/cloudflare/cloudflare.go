package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Cloudflare IP ranges URLs
const (
	IPv4URL = "https://www.cloudflare.com/ips-v4/"
	IPv6URL = "https://www.cloudflare.com/ips-v6/"
)

var (
	IPNetsReloadInterval = 24 * time.Hour // Default reload interval
)

// Default static Cloudflare IP ranges as fallback
// These ranges are from https://www.cloudflare.com/ips/
var DefaultIPNets = []string{
	// IPv4 ranges
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",

	// IPv6 ranges
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

// Cloudflare manages Cloudflare IP ranges and provides IP checking functionality
type Cloudflare struct {
	latestNets  atomic.Value // stores *cloudflareData
	defaultNets IPNets
}

// New creates a new CloudflareIPChecker instance
func New(ctx context.Context) *Cloudflare {
	c := &Cloudflare{
		defaultNets: toIPNets(DefaultIPNets),
	}

	go c.reload(ctx)

	return c
}

func (c *Cloudflare) reload(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(IPNetsReloadInterval):
			ranges := []string{}

			v4, err := c.fetch(IPv4URL)
			if err == nil {
				ranges = append(ranges, v4...)
			}

			v6, err := c.fetch(IPv6URL)
			if err == nil {
				ranges = append(ranges, v6...)
			}

			// If we couldn't fetch from API, use fallback ranges
			if len(ranges) == 0 {
				return
			}

			c.latestNets.Store(toIPNets(ranges))
		}
	}
}

// fetch fetches IP ranges from a given URL
func (c *Cloudflare) fetch(url string) ([]string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	var ranges []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ranges = append(ranges, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", url, err)
	}

	return ranges, nil
}

// Contains checks if an IP address is within Cloudflare's IP ranges
func (c *Cloudflare) Contains(s string) bool {
	data := c.latestNets.Load()
	if data == nil {
		return c.defaultNets.Contains(s)
	}

	nets := data.(IPNets)

	return nets.Contains(s)
}

func (c *Cloudflare) GetVisitor(r *http.Request) (string, string) {
	// ip, _, err := net.SplitHostPort(r.RemoteAddr)
	// if err != nil {
	// 	return r.RemoteAddr, ""
	// }

	// if c.Contains(ip) {
	country := r.Header.Get("Cf-Ipcountry")

	if r.Header.Get("Cf-Pseudo-Ipv4") != "" {
		return r.Header.Get("Cf-Connecting-Ipv6"), country
	} else {
		return r.Header.Get("Cf-Connecting-Ip"), country
	}
	// }

	// return ip, ""
}
