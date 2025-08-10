package sse

import "time"

type Option func(*Server)

func WithClientTimeout(d time.Duration) Option {
	return func(s *Server) {
		if d > 0 {
			s.clientTimeout = d
		}
	}
}
