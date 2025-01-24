package hsts

import "time"

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
