package proxyproto

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"sync"
)

var Logger = log.Default()

type conn struct {
	net.Conn
	r *bufio.Reader
	h *Header

	isLoaded bool
	once     sync.Once
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
	if !c.isLoaded {
		c.once.Do(c.tryUseProxy)
	}
	return c.r.Read(b)
}

// LocalAddr returns the local network address, if known.
func (c *conn) LocalAddr() net.Addr {
	if !c.isLoaded {
		c.once.Do(c.tryUseProxy)
	}

	if c.h != nil {
		return c.h.LocalAddr
	}
	return c.Conn.LocalAddr()
}

// RemoteAddr returns the remote network address, if known.
func (c *conn) RemoteAddr() net.Addr {
	if !c.isLoaded {
		c.once.Do(c.tryUseProxy)
	}
	if c.h != nil {
		return c.h.RemoteAddr
	}
	return c.Conn.RemoteAddr()
}

// tryUseProxy tries to read the PROXY protocol header from the connection.
// If the header is read successfully, it sets the Header field and marks the
// connection as proxied. If the header is invalid or not present, it does
// nothing.
func (c *conn) tryUseProxy() {
	defer func() {
		c.isLoaded = true
	}()
	// Read the first 13 bytes which should contain the identifier
	buf, err := c.r.Peek(13)
	if err != nil {
		Logger.Println("proxyproto: peek", err)
		return
	}

	// v1
	if bytes.HasPrefix(buf[0:13], v1) {
		c.h = readV1Header(c.r)
	} else if bytes.HasPrefix(buf[0:13], v2) {
		c.h = readV2Header(c.r)
	}
}

func (c *conn) String() string {
	if c.h != nil {
		return fmt.Sprintf("PROXYv%v %v", c.h.Protocol, c.Conn)
	}
	return fmt.Sprintf("%v", c.Conn)
}
