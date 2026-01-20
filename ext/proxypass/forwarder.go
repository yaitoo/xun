package proxypass

import (
	"net"
	"net/http"
)

var forwarders map[string]Forwarder = make(map[string]Forwarder)

type Forwarder interface {
	Name() string
	GetVisitor(r *http.Request) (string, string)
}

func RegisterForwarder(name string, f Forwarder) {
	forwarders[name] = f
}

type NoForwarder struct {
}

func (f *NoForwarder) Name() string {
	return ""
}

func (f *NoForwarder) GetVisitor(r *http.Request) (string, string) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr, ""
	}

	return ip, ""
}
