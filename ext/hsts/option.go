package hsts

import (
	"net/http"
	"strings"
	"time"
)

// Config represents the configuration options for HSTS (HTTP Strict Transport Security).
// It includes various settings such as MaxAge, IncludeSubDomains, and Preload.
type Config struct {
	MaxAge            int64 // MaxAge specifies the duration for which the HSTS policy is in effect.
	IncludeSubDomains bool  // IncludeSubDomains indicates whether the HSTS policy applies to subdomains.
	Preload           bool  // Preload indicates whether the domain should be preloaded into browsers' HSTS lists.
}

// Option is a function that modifies a Config instance.
// It is used to configure HSTS settings by applying various options.
type Option func(c *Config)

// WithMaxAge sets the maximum age for the HSTS policy.
//
// The maximum age specifies the duration for which the HSTS policy is in effect.
// Note that the maximum age is specified in seconds, so "1h" would be equivalent to 3600.
func WithMaxAge(t time.Duration) Option {
	return func(c *Config) {
		if t > 0 {
			c.MaxAge = int64(t / time.Second)
		}
	}
}

// WithIncludeSubDomains sets the HSTS policy applies to subdomains.
func WithIncludeSubDomains() Option {
	return func(c *Config) {
		c.IncludeSubDomains = true
	}
}

// WithPreload sets the domain should be preloaded into browsers' HSTS lists.
func WithPreload() Option {
	return func(c *Config) {
		c.Preload = true
	}
}

// IgnoreRule is a function that takes a pointer to an http.Request
// and returns a boolean indicating whether the request should be
// ignored by the HSTS middleware.
type IgnoreRule func(*http.Request) bool

// Match creates an IgnoreRule that matches the given paths to ignore requests.
//
// The paths are matched case-insensitively, so "/Doc" and "/doc" would be equivalent.
func Match(paths ...string) IgnoreRule {
	return func(r *http.Request) bool {
		for _, path := range paths {
			if strings.EqualFold(r.URL.Path, path) {
				return true
			}
		}
		return false
	}
}

// StartsWith creates an IgnoreRule that checks if the request path starts with any of the specified prefixes.
//
// paths MUST be lower case.
//
// This function returns an IgnoreRule that converts the request path to lowercase and checks for a prefix match.
// It is useful for ignoring requests with paths that match specified patterns.
func StartsWith(paths ...string) IgnoreRule {
	return func(r *http.Request) bool {
		l := strings.ToLower(r.URL.Path)
		for _, path := range paths {
			if strings.HasPrefix(l, path) {
				return true
			}
		}
		return false
	}
}
