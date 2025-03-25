package cache

import (
	"strings"
	"time"
)

type Rule struct {
	StartsWith string
	EndsWith   string
	Duration   time.Duration
}

func (r *Rule) Match(path string) bool {
	if r.StartsWith != "" && !hasPrefix(path, r.StartsWith) {
		return false
	}

	if r.EndsWith != "" && !hasSuffix(path, r.EndsWith) {
		return false
	}

	return true
}

type Options struct {
	Rules []Rule
}

type Option func(o *Options)

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

func hasPrefix(s string, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

func hasSuffix(s string, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return strings.EqualFold(s[len(s)-len(suffix):], suffix)
}
