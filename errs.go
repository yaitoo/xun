package xun

import "errors"

var (
	ErrCancelled    = errors.New("xun: request_cancelled")
	ErrViewNotFound = errors.New("xun: view_not_found")
)
