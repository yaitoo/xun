package sse

type Error struct {
	ClientID string
	error
}

func (e *Error) Unwrap() error {
	return e.error
}
