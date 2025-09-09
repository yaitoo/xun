package proxypass

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/yaitoo/xun"
)

func HandleConnect(c *xun.Context) error {
	code := c.Request.Header.Get("X-Code")
	domain := strings.Split(c.Request.Header.Get("X-Domain"), ",")
	target, _ := url.Parse(c.Request.Header.Get("X-Target"))

	hostname := target.Hostname()

	if hostname == "0.0.0.0" {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		port := target.Port()
		if port == "" {
			target.Host = ip
		} else {
			target.Host = net.JoinHostPort(ip, target.Port())
		}
	}

	s := create(target)

	client, cid, _, err := connections.Join(code, c.Response)
	if err != nil {
		c.WriteStatus(http.StatusBadRequest)
		return xun.ErrCancelled
	}

	log.Println("proxypass: online", code, domain, "=>", target.String())
	for _, d := range domain {
		up(d, s)
	}

	err = client.Wait(c.Request.Context()) //nolint: errcheck

	for _, d := range domain {
		down(d)
	}

	connections.Leave(code, cid)
	client.Close()

	log.Println("proxypass: offline", code, domain, err)

	return xun.ErrCancelled
}
