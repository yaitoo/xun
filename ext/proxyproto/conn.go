package proxyproto

import (
	"bufio"
	"fmt"
	"net"
)

type conn struct {
	net.Conn

	r *bufio.Reader

	h *Header
}

func NewConn(nc net.Conn) (net.Conn, error) {
	c := &conn{Conn: nc, r: bufio.NewReader(nc)}
	if err := c.Proxy(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *conn) LocalAddr() net.Addr {
	if c.h != nil {
		return c.h.Source
	}
	return c.Conn.LocalAddr()
}
func (c *conn) RemoteAddr() net.Addr {
	if c.h != nil {
		return c.h.Destination
	}
	return c.Conn.RemoteAddr()
}

func (c *conn) Proxy() error {
	var err error
	c.h, err = ReadHeader(c.r)
	return err
}

func (c *conn) String() string {
	if c.h != nil {
		return fmt.Sprintf("proxied connection %v", c.Conn)
	}
	return fmt.Sprintf("%v", c.Conn)
}
