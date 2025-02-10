package proxyproto

import "errors"

var (
	ErrInvalidProxyHeader = errors.New("proxyproto: invalid_proxy_header")
	ErrUnknownProtocol    = errors.New("proxyproto: unknown_protocol")
)
