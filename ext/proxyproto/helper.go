package proxyproto

import (
	"bytes"
	"net"
)

func newIPAddr(transport Protocol, ip net.IP, port uint16) net.Addr {
	if transport.IsStream() {
		return &net.TCPAddr{IP: ip, Port: int(port)}
	} else if transport.IsDatagram() {
		return &net.UDPAddr{IP: ip, Port: int(port)}
	} else {
		return nil
	}
}

func parseUnixName(b []byte) string {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return string(b)
	}
	return string(b[:i])
}
