package proxyproto

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
)

type conn struct {
	net.Conn
	r *bufio.Reader
	h *Header
}

// NewConn wraps a net.Conn and returns a new proxyproto.Conn that reads the
// PROXY protocol header from the connection. If the connection is not a
// PROXY protocol connection, it returns the original connection.
func NewConn(nc net.Conn) (net.Conn, error) {
	c := &conn{Conn: nc, r: bufio.NewReader(nc)}
	if err := c.Proxy(); err != nil {
		slog.Info("", slog.Any("err", err))
		return nil, err
	}
	return c, nil
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *conn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

// LocalAddr returns the local network address, if known.
func (c *conn) LocalAddr() net.Addr {
	if c.h != nil {
		return c.h.LocalAddr
	}
	return c.Conn.LocalAddr()
}

// RemoteAddr returns the remote network address, if known.
func (c *conn) RemoteAddr() net.Addr {
	if c.h != nil {
		return c.h.RemoteAddr
	}
	return c.Conn.RemoteAddr()
}

func (c *conn) Proxy() error {
	var err error
	c.h, err = ReadHeader(c.r)
	return err
}

func (c *conn) Close() error {
	return c.Conn.Close()
}

func (c *conn) String() string {
	if c.h != nil {
		return fmt.Sprintf("proxied connection %v", c.Conn)
	}
	return fmt.Sprintf("%v", c.Conn)
}
