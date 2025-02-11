package proxyproto

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockConn struct {
	bytes      []byte
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (m *mockConn) Read(p []byte) (n int, err error) {

	copy(p, m.bytes)

	if len(m.bytes) < len(p) {
		return len(m.bytes), io.EOF
	}
	return len(p), nil
}
func (m *mockConn) Write(p []byte) (n int, err error) { panic("not implemented") }
func (m *mockConn) Close() error                      { panic("not implemented") }
func (m *mockConn) LocalAddr() net.Addr               { return m.localAddr }
func (m *mockConn) RemoteAddr() net.Addr              { return m.remoteAddr }
func (m *mockConn) SetDeadline(time.Time) error       { panic("not implemented") }
func (m *mockConn) SetReadDeadline(time.Time) error   { panic("not implemented") }
func (m *mockConn) SetWriteDeadline(time.Time) error  { panic("not implemented") }

func TestConn(t *testing.T) {

	tests := []struct {
		name       string
		mc         *mockConn
		fireFunc   func(c net.Conn)
		remoteAddr net.Addr
		localAddr  net.Addr
		err        bool
	}{
		{
			name: "v1/read_first",
			mc: &mockConn{
				bytes:      []byte("PROXY TCP4 192.168.0.1 192.168.0.2 56324 443\r\n"),
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.Read(make([]byte, 1)) // nolint: errcheck
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
		},
		{
			name: "v1/remote_first",
			mc: &mockConn{
				bytes:      []byte("PROXY TCP4 192.168.0.1 192.168.0.2 56324 443\r\n"),
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.RemoteAddr()
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
		},
		{
			name: "v1/local_first",
			mc: &mockConn{
				bytes:      []byte("PROXY TCP4 192.168.0.1 192.168.0.2 56324 443\r\n"),
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.LocalAddr()
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
		},
		{
			name: "v2/read_first",
			mc: &mockConn{bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
				// IPV4 -------------|  IPV4 ----------------|   SRC PORT   DEST PORT
				0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0xCA, 0x2B, 0x04, 0x01},
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.Read(make([]byte, 1)) // nolint: errcheck
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51755},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1025},
		},
		{
			name: "v2/remote_first",
			mc: &mockConn{bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
				// IPV4 -------------|  IPV4 ----------------|   SRC PORT   DEST PORT
				0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0xCA, 0x2B, 0x04, 0x01},
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.RemoteAddr()
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51755},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1025},
		},
		{
			name: "v2/local_first",
			mc: &mockConn{bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
				// IPV4 -------------|  IPV4 ----------------|   SRC PORT   DEST PORT
				0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0xCA, 0x2B, 0x04, 0x01},
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			fireFunc: func(c net.Conn) {
				c.LocalAddr()
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51755},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1025},
		},
		{
			name: "no_proxyproto",
			mc: &mockConn{
				bytes:      []byte("PROXY TCP4\r\n"),
				localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			},
			remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 56324},
			localAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.0.2"), Port: 443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewConn(tt.mc)
			if tt.fireFunc != nil {
				tt.fireFunc(mc)
			}

			require.Equal(t, tt.remoteAddr.String(), mc.RemoteAddr().String())
			require.Equal(t, tt.localAddr.String(), mc.LocalAddr().String())

		})
	}
}
