package proxyproto

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

type conn struct {
	net.Conn
	r         *bufio.Reader
	h         *Header
	isProxied bool
	once      sync.Once
}

// NewConn wraps a net.Conn and returns a new proxyproto.Conn that reads the
// PROXY protocol header from the connection. If the connection is not a
// PROXY protocol connection, it returns the original connection.
func NewConn(nc net.Conn) (net.Conn, error) {
	return &conn{Conn: nc, r: bufio.NewReader(nc)}, nil
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *conn) Read(b []byte) (n int, err error) {
	if !c.isProxied {
		c.once.Do(c.Proxy)
		c.isProxied = true
	}
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

func (c *conn) Proxy() {
	// For v1 the header length is at most 108 bytes.
	// For v2 the header length is at most 52 bytes plus the length of the TLVs.
	// We use 256 bytes to be safe.

	// Read the first 13 bytes which should contain the identifier
	buf, err := c.r.Peek(13)
	if err != nil {
		slog.Info("proxyproto: read header", slog.Any("err", err))
		return
	}

	// v1
	if bytes.HasPrefix(buf[0:13], v1) {
		c.h, err = readV1Header(c.r)

		if err != nil {
			slog.Info("", slog.Any("err", err))
			return
		}
	}

	// v2
	if bytes.HasPrefix(buf[0:13], v2) {
		c.h, err = readV2Header(c.r)
		if err != nil {
			slog.Info("", slog.Any("err", err))
			return
		}
	}
}

func (c *conn) Close() error {
	return c.Conn.Close()
}

func (c *conn) String() string {
	if c.h != nil {
		return fmt.Sprintf("proxyproto connection %v", c.Conn)
	}
	return fmt.Sprintf("%v", c.Conn)
}
