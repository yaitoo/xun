package proxyproto

import (
	"net"
)

type listener struct {
	net.Listener
}

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewConn(c)
}

// NewListener wraps a net.Listener and returns a new net.Listener that returns
// a proxyproto.Conn when Accept is called.
//
// It is used to handle PROXY protocol connections.
func NewListener(l net.Listener) net.Listener {
	return &listener{Listener: l}
}
