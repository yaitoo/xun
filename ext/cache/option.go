package cache

import (
	"strings"
	"time"
)

// Rule defines a caching rule with path matching criteria and cache duration.
// It can match request paths by prefix, suffix, or both.
type Rule struct {
	StartsWith string        // Path prefix to match
	EndsWith   string        // Path suffix to match
	Duration   time.Duration // Cache duration to apply
}

// Match checks if the provided path matches the rule's criteria.
// A path matches when it satisfies both StartsWith and EndsWith conditions.
func (r *Rule) Match(path string) bool {
	if r.StartsWith != "" && !hasPrefix(path, r.StartsWith) {
		return false
	}

	if r.EndsWith != "" && !hasSuffix(path, r.EndsWith) {
		return false
	}

	return true
}

// Options stores configuration for the cache middleware.
type Options struct {
	Rules []Rule // Collection of caching rules to apply
}

// Option is a function that configures the cache Options.
type Option func(o *Options)

// Match creates an Option that adds a new caching rule.
// The rule matches paths that start with 'startsWith' and end with 'endsWith',
// applying the specified cache duration.
func Match(startsWith, endsWith string, duration time.Duration) Option {
	return func(o *Options) {
		if (startsWith != "" || endsWith != "") && duration > 0 {
			o.Rules = append(o.Rules, Rule{
				StartsWith: startsWith,
				EndsWith:   endsWith,
				Duration:   duration,
			})
		}
	}
}

// hasPrefix checks if string s starts with prefix, ignoring case.
func hasPrefix(s string, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

// hasSuffix checks if string s ends with suffix, ignoring case.
func hasSuffix(s string, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return strings.EqualFold(s[len(s)-len(suffix):], suffix)
}
