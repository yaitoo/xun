package proxypass

import (
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/yaitoo/xun"
)

func HandleConnect(c *xun.Context) error {
	code := c.Request.Header.Get("X-Code")
	domain := c.Request.Header.Get("X-Domain")
	target, _ := url.Parse(c.Request.Header.Get("X-Target"))

	if target.Host == "0.0.0.0" {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		port := target.Port()
		if port == "" {
			target.Host = ip
		} else {
			target.Host = net.JoinHostPort(ip, target.Port())
		}
	}

	s := create(domain, target)

	client, cid, _, err := connections.Join(code, c.Response)
	if err != nil {
		c.WriteStatus(http.StatusBadRequest)
		return xun.ErrCancelled
	}

	log.Println("conn: online", domain, "=>", target.String())
	up(domain, s)
	err = client.Wait(c.Request.Context()) //nolint: errcheck
	down(domain)
	connections.Leave(code, cid)
	client.Close()

	log.Println("server: offline", domain, err)

	return xun.ErrCancelled
}
