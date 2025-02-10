package proxyproto

import "errors"

var (
	ErrInvalidHeader   = errors.New("proxyproto: invalid_header")
	ErrInvalidProtocol = errors.New("proxyproto: invalid_protocol")
)
