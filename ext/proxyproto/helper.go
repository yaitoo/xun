package proxyproto

import (
	"net"
)

func toAddr(transport Protocol, ip net.IP, port uint16) net.Addr {
	if transport.IsStream() {
		return &net.TCPAddr{IP: ip, Port: int(port)}
	}
	if transport.IsDatagram() {
		return &net.UDPAddr{IP: ip, Port: int(port)}
	}
	return nil
}
