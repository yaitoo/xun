package htmx

import "errors"

var (
	ErrCancelled    = errors.New("htmx: cancelled")
	ErrViewNotFound = errors.New("htmx: view_not_found")
)
