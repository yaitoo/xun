package sse

type Error struct {
	ClientID string
	error
}

func (e *Error) Unwrap() error {
	return e.error
}

func NewError(clientID string, err error) *Error {
	return &Error{
		ClientID: clientID,
		error:    err,
	}
}
