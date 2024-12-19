package htmx

import "errors"

var (
	ErrHandleCancelled = errors.New("htmx: handle_cancelled")
	ErrViewNotFound    = errors.New("htmx: view_not_found")
)
