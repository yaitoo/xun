package proxyproto

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestV1(t *testing.T) {
	tests := []struct {
		name   string
		header string
		remote net.Addr
		local  net.Addr
		err    bool
	}{
		{
			name:   "TCP4/minimal",
			header: "PROXY TCP4 1.1.1.5 1.1.1.6 2 3\r\n",
			remote: &net.TCPAddr{IP: net.ParseIP("1.1.1.5"), Port: 2},
			local:  &net.TCPAddr{IP: net.ParseIP("1.1.1.6"), Port: 3},
		},
		{
			name:   "TCP4/maximal",
			header: "PROXY TCP4 255.255.255.255 255.255.255.254 65535 65535\r\n",
			remote: &net.TCPAddr{IP: net.ParseIP("255.255.255.255"), Port: 65535},
			local:  &net.TCPAddr{IP: net.ParseIP("255.255.255.254"), Port: 65535},
		},
		{
			name:   "TCP6/minimal",
			header: "PROXY TCP6 ::1 ::2 3 4\r\n",
			remote: &net.TCPAddr{IP: net.ParseIP("::1"), Port: 3},
			local:  &net.TCPAddr{IP: net.ParseIP("::2"), Port: 4},
		},
		{
			name:   "TCP6/maximal",
			header: "PROXY TCP6 0000:0000:0000:0000:0000:0000:0000:0002 0000:0000:0000:0000:0000:0000:0000:0001 65535 65535\r\n",
			remote: &net.TCPAddr{IP: net.ParseIP("0000:0000:0000:0000:0000:0000:0000:0002"), Port: 65535},
			local:  &net.TCPAddr{IP: net.ParseIP("0000:0000:0000:0000:0000:0000:0000:0001"), Port: 65535},
		},
		{
			name:   "UNKNOWN/minimal",
			header: "PROXY UNKNOWN\r\n",
			err:    true,
		},
		{
			name:   "UNKNOWN/maximal",
			header: "PROXY UNKNOWN 0000:0000:0000:0000:0000:0000:0000:0002 0000:0000:0000:0000:0000:0000:0000:0001 65535 65535\r\n",
			err:    true,
		},
		{
			name:   "TCP6/empty",
			header: "PROXY TCP6\r\n",
			remote: &net.TCPAddr{IP: net.ParseIP("::1"), Port: 3},
			local:  &net.TCPAddr{IP: net.ParseIP("::2"), Port: 4},
			err:    true,
		},
		{
			name:   "TCP6/cRLF_not_found",
			header: "PROXY TCP6 0000:0000:0000:0000:0000:0000:0000:0001 0000:0000:0000:0000:0000:0000:0000:0001 65535 65535XXXX\r\n",
			err:    true,
		},
		{
			name:   "UNKNOWN/cRLF_not_found",
			header: "PROXY UNKNOWN 0000:0000:0000:0000:0000:0000:0000:0001 0000:0000:0000:0000:0000:0000:0000:0001 65535 65535X\r\n",
			err:    true,
		},
		{
			name:   "UNKNOWN/no_cRLF",
			header: "PROXY UNKNOWN",
			err:    true,
		},
		{
			name:   "Header/only_cRLF",
			header: "\r\n",
			err:    true,
		},
		{
			name:   "Header/empty",
			header: "",
			err:    true,
		},
		{
			name:   "Header/garbage",
			header: "ASDFASDGSAG@#!@#$!WDFGASDGASDFG#TAGASDFASDG@",
			err:    true,
		},
		{
			name:   "TCP4/incomplete",
			header: "PROXY TCP4 garbage\r\n",
			err:    true,
		},
		{
			name:   "TCP6/incomplete",
			header: "PROXY TCP6 garbage\r\n",
			err:    true,
		},
		{
			name:   "PROTO/unrecognized",
			header: "PROXY UNIX :1 :1 234 234\r\n",
			err:    true,
		},
		{
			name:   "TCP4/invalid_src",
			header: "PROXY TCP4 NOT-AN-IP 192.168.1.1 22 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP4/invalid_dest",
			header: "PROXY TCP4 192.168.1.1 NOT-AN-IP 22 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP4/invalid_src_port",
			header: "PROXY TCP4 192.168.1.1 192.168.1.1 NOT-A-PORT 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP4/invalid_dest_port",
			header: "PROXY TCP4 192.168.1.1 192.168.1.1 22 NOT-A-PORT\r\n",
			err:    true,
		},
		{
			name:   "TCP4/corrupted_address_line",
			header: "PROXY TCP4 192.168.1.1 192.168.1.1 2345\r\n",
			err:    true,
		},

		{
			name:   "TCP6/invalid_src",
			header: "PROXY TCP6 NOT-AN-IP ::1 22 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP6/invalid_dest",
			header: "PROXY TCP6 NOT-AN-IP ::1 22 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP6/invalid_src_port",
			header: "PROXY TCP6 ::1 ::2 NOT-A-PORT 2345\r\n",
			err:    true,
		},
		{
			name:   "TCP6/invalid_dest_port",
			header: "PROXY TCP6 ::1 ::2 22 NOT-A-PORT\r\n",
			err:    true,
		},
		{
			name:   "TCP6/corrupted_address_line",
			header: "PROXY TCP6 ::1 ::2 2345\r\n",
			err:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.header))
			h := readV1Header(r)

			if tt.err {
				require.Nil(t, h)
			} else {
				require.Equal(t, tt.local, h.LocalAddr)
				require.Equal(t, tt.remote, h.RemoteAddr)
				require.Equal(t, 1, h.Version)
			}
		})
	}
}

func TestV2(t *testing.T) {
	tests := []struct {
		name    string
		header  []byte
		remote  net.Addr
		local   net.Addr
		rawTLVs []byte
		err     bool
	}{
		{
			name: "TCP4/127.0.0.1",
			//                                                                                     VER  IP/TCP LENGTH
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
				// IPV4 -------------|  IPV4 ----------------|   SRC PORT   DEST PORT
				0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0xCA, 0x2B, 0x04, 0x01},
			remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51755},
			local:  &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 1025},
		},
		{
			name: "UDP4/127.0.0.1",
			//                                                                                          IP/UDP
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x12, 0x00, 0x0C,
				0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x01, 0xCA, 0x2B, 0x04, 0x01},
			remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51755},
			local:  &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1025},
		},
		{
			name: "TCP6/127.0.0.1",
			//                                                                                     VER  IP/TCP   LENGTH
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x24,
				// IPV6 -------------------------------------------------------------------------------------|
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0x7F, 0x00, 0x00, 0x01,
				// IPV6 -------------------------------------------------------------------------------------|   SRC PORT   DEST PORT
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0x7F, 0x00, 0x00, 0x01, 0xCC, 0x4C, 0x04, 0x01},
			remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 52300},
			local:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1025},
		},
		{
			name: "TCP6/maximal",
			//                                                                                     VER  IP/TCP   LENGTH
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x24,
				// IPV6 -------------------------------------------------------------------------------------|
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				// IPV6 -------------------------------------------------------------------------------------|   SRC PORT   DEST PORT
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			remote: &net.TCPAddr{IP: net.ParseIP("FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF"), Port: 65535},
			local:  &net.TCPAddr{IP: net.ParseIP("FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF"), Port: 65535},
		},
		{
			name: "TCP6/::1",
			//                                                                                     VER  IP/TCP   LENGTH
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x2B,
				// IPV6 -------------------------------------------------------------------------------------|
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// IPV6 -------------------------------------------------------------------------------------|   SRC PORT   DEST PORT
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xCF, 0x8F, 0x04, 0x01,
				// TLVs
				0x03, 0x00, 0x04, 0xFD, 0x16, 0xEE, 0x60},
			remote:  &net.TCPAddr{IP: net.ParseIP("::1"), Port: 53135},
			local:   &net.TCPAddr{IP: net.ParseIP("::1"), Port: 1025},
			rawTLVs: []byte{0x03, 0x00, 0x04, 0xFD, 0x16, 0xEE, 0x60},
		},
		{
			name: "UDP6/::1",
			//                                                                                     VER  IP/TCP   LENGTH
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x22, 0x00, 0x2B,
				// IPV6 -------------------------------------------------------------------------------------|
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// IPV6 -------------------------------------------------------------------------------------|   SRC PORT   DEST PORT
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xCF, 0x8F, 0x04, 0x01,
				// TLVs
				0x03, 0x00, 0x04, 0xFD, 0x16, 0xEE, 0x60},
			remote:  &net.UDPAddr{IP: net.ParseIP("::1"), Port: 53135},
			local:   &net.UDPAddr{IP: net.ParseIP("::1"), Port: 1025},
			rawTLVs: []byte{0x03, 0x00, 0x04, 0xFD, 0x16, 0xEE, 0x60},
		},
		{
			name:   "invalid/missing_proto_family_length",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21},
			err:    true,
		},
		{
			name:   "invalid/version",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x01, 0x21, 0x00, 0x2B},
			err:    true,
		},
		{
			name:   "invalid/length_too_long",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x08, 0x01},
			err:    true,
		},
		{
			name:   "invalid/too_less_bytes",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x08, 0x00},
			err:    true,
		},
		{
			name:   "invalid/local_with_no_trailing bytes",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x20, 0x00, 0x00, 0x00},
			err:    true,
		},
		{
			name: "invalid/local_with_trailing_bytes_TLVs",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x20, 0xFF, 0x00, 0x07,
				0x03, 0x00, 0x04, 0xFD, 0x16, 0xEE, 0x60},
			err: true,
		},
		{
			name:   "invalid/proxy_with_zero_byte_header",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x00, 0x00, 0x00},
			err:    true,
		},
		{
			name:   "invalid/invalid-length_for_IPv4",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x01, 0xFF},
			err:    true,
		},
		{
			name:   "invalid/invalid_length_for_IPv6",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x01, 0xFF},
			err:    true,
		},
		{
			name:   "invalid/unix_socket_not_implemented",
			header: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x31, 0x00, 0x01, 0xFF},
			err:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(bytes.NewReader(tt.header))
			h := readV2Header(r)

			if tt.err {
				require.Nil(t, h)
			} else {
				require.Equal(t, tt.local.String(), h.LocalAddr.String())
				require.Equal(t, tt.remote.String(), h.RemoteAddr.String())
				require.Equal(t, tt.rawTLVs, h.RawTLVs)
				require.Equal(t, 2, h.Version)
			}

		})
	}
}

func TestBrokenReader(t *testing.T) {

	tests := []struct {
		name  string
		bytes []byte
		read  func(i int, p []byte) (n int, err error)
	}{
		{
			name: "break_on_first_12_bytes",
		},
		{
			name: "break_on_13_byte",

			bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A},
		},
		{
			name:  "break_on_14_byte",
			bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21},
		},
		{
			name:  "break_on_16_byte",
			bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11},
		},
		{
			name:  "invalid_on_14_byte",
			bytes: []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x01},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := readV2Header(bufio.NewReader(bytes.NewReader(test.bytes)))

			require.Nil(t, h)
		})
	}
}
