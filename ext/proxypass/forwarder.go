package proxypass

import "github.com/yaitoo/xun"

type Forwarder interface {
	GetVisitor(c *xun.Context) (string, string)
}
