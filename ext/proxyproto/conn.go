package proxyproto

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type conn struct {
	cn      net.Conn
	r       *bufio.Reader
	local   net.Addr
	remote  net.Addr
	proxied bool
}

func NewProxyConn(cn net.Conn) (net.Conn, error) {
	c := &conn{cn: cn, r: bufio.NewReader(cn)}
	if err := c.Init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *conn) Close() error                { return c.cn.Close() }
func (c *conn) Write(b []byte) (int, error) { return c.cn.Write(b) }

func (c *conn) SetDeadline(t time.Time) error      { return c.cn.SetDeadline(t) }
func (c *conn) SetReadDeadline(t time.Time) error  { return c.cn.SetReadDeadline(t) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.cn.SetWriteDeadline(t) }

func (c *conn) LocalAddr() net.Addr  { return c.local }
func (c *conn) RemoteAddr() net.Addr { return c.remote }

func (c *conn) Read(b []byte) (int, error) { return c.r.Read(b) }

func (c *conn) Init() error {
	buf, err := c.r.Peek(5)
	if err != io.EOF && err != nil {
		return err
	}

	if err == nil && bytes.Equal([]byte(`PROXY`), buf) {
		c.proxied = true
		proxyLine, err := c.r.ReadString('\n')
		if err != nil {
			return err
		}
		fields := strings.Fields(proxyLine)
		c.remote = &addr{net.JoinHostPort(fields[2], fields[4])}
		c.local = &addr{net.JoinHostPort(fields[3], fields[5])}
	} else {
		c.local = c.cn.LocalAddr()
		c.remote = c.cn.RemoteAddr()
	}

	return nil
}

func (c *conn) String() string {
	if c.proxied {
		return fmt.Sprintf("proxied connection %v", c.cn)
	}
	return fmt.Sprintf("%v", c.cn)
}

type addr struct{ hp string }

func (a addr) Network() string { return "tcp" }
func (a addr) String() string  { return a.hp }
