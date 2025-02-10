package proxyproto

import (
	"log/slog"
	"net"
)

type listener struct {
	net.Listener
}

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		slog.Info("proxyproto: accept", slog.Any("err", err))
		return nil, err
	}
	return NewConn(c)
}

func NewListener(l net.Listener) net.Listener {
	return &listener{Listener: l}
}
