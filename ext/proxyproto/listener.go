package proxyproto

import "net"

type Listener struct {
	net.Listener
}

func (l *Listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewConn(c)
}

func NewListener(l net.Listener) *Listener {
	return &Listener{Listener: l}
}
