package htmx

import "errors"

var (
	ErrCancelled = errors.New("htmx: request_cancelled")
)
