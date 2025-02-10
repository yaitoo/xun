package proxyproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

type Header struct {
	// LocalAddr is the ip address of the party that initiated the connection
	LocalAddr net.Addr
	// RemoteAddr is the ip address the remote party connected to; aka the address
	// the proxy was listening for connections on.
	RemoteAddr net.Addr
	// The version of the proxy protocol parsed
	Version int

	// V2
	Command  Command
	Protocol Protocol
	// The unparsed TLVs (Type-Length-Value) that were appended to the end of
	// the v2 proto proxy header.
	RawTLVs []byte
}

var (
	lengthUnspec = uint16(0)
	lengthV4     = uint16(12)
	lengthV6     = uint16(36)
	lengthUnix   = uint16(216)
)

func (header *Header) validateLength(length uint16) bool {
	if header.Protocol.IsIPv4() {
		return length >= lengthV4
	} else if header.Protocol.IsIPv6() {
		return length >= lengthV6
	} else if header.Protocol.IsUnix() {
		return length >= lengthUnix
	} else if header.Protocol.IsUnspec() {
		return length >= lengthUnspec
	}
	return false
}

var (
	v1 = []byte("PROXY ")
	v2 = []byte("\r\n\r\n\x00\r\nQUIT\n")
)

const (
	v1_Unknown = "UNKNOWN"
	v1_TCP6    = "TCP6"
	v1_TCP4    = "TCP4"
)

func ReadHeader(r *bufio.Reader) (*Header, error) {
	// For v1 the header length is at most 108 bytes.
	// For v2 the header length is at most 52 bytes plus the length of the TLVs.
	// We use 256 bytes to be safe.

	// Read the first 13 bytes which should contain the identifier
	buf, err := r.Peek(13)
	if err != nil {
		return nil, fmt.Errorf("proxyproto: read header %w", err)
	}

	// v1
	if bytes.HasPrefix(buf[0:13], v1) {
		return readV1Header(r)
	}

	// v2
	if bytes.HasPrefix(buf[0:13], v2) {
		return readV2Header(r)
	}

	return nil, nil
}

func readV1Header(r *bufio.Reader) (*Header, error) {
	proxyLine, err := r.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("proxyproto: read v1 header %w", err)
	}

	// PROXY + Protocol + src_ip + dest_ip + src_port + dest_port
	// PROXY TCP4 192.168.0.1 192.168.0.10 12345 80\r\n
	// PROXY TCP6 2001:db8::1 2001:db8::100 12345 80\r\n
	// PROXY UNKNOWN\r\n
	fields := strings.Fields(proxyLine)

	if len(fields) < 2 {
		return nil, ErrInvalidHeader
	}

	if fields[1] == v1_TCP4 || fields[1] == v1_TCP6 {
		h := &Header{}
		h.Version = 1

		var err error
		h.RemoteAddr, err = net.ResolveTCPAddr("tcp", fields[2]+":"+fields[4])
		if err != nil {
			return nil, fmt.Errorf("proxyproto: invalid remote address: %w", err)
		}
		h.LocalAddr, err = net.ResolveTCPAddr("tcp", fields[3]+":"+fields[5])
		if err != nil {
			return nil, fmt.Errorf("proxyproto: invalid local address: %w", err)
		}
		return h, nil
	}

	return nil, ErrInvalidProtocol
}

type Addr4 struct {
	Remote     [4]byte
	Local      [4]byte
	RemotePort uint16
	LocalPort  uint16
}

type Addr6 struct {
	Remote     [16]byte
	Local      [16]byte
	RemotePort uint16
	LocalPort  uint16
}

type AddrUnix struct {
	Remote [108]byte
	Local  [108]byte
}

// readV2Header assumes the passed buf contains the first 13 bytes which should look like
// the following. (Where X is the proto proxy version and command)
//
//	"\r\n\r\n\x00\r\nQUIT\nX"
func readV2Header(reader *bufio.Reader) (*Header, error) {
	var err error
	// Skip first 12 bytes (signature)
	for i := 0; i < 12; i++ {
		if _, err = reader.ReadByte(); err != nil {
			return nil, fmt.Errorf("proxyproto: read v2 signature: %w", err)
		}
	}

	header := new(Header)
	header.Version = 2

	// Read the 13th byte, protocol version and command
	b13, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("proxyproto: read v2 command: %w", err)
	}
	header.Command = Command(b13)
	if _, ok := supportedCommand[header.Command]; !ok {
		return nil, fmt.Errorf("proxyproto: invalid command: %w", err)
	}

	// Read the 14th byte, address family and protocol
	b14, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("proxyproto: read v2 protocol: %w", err)
	}
	header.Protocol = Protocol(b14)
	// UNSPEC is only supported when LOCAL is set.
	if header.Protocol == UNSPEC && header.Command != LOCAL {
		return nil, fmt.Errorf("proxyproto: invalid protocol: %w", err)
	}

	// Make sure there are bytes available as specified in length
	var length uint16
	if err := binary.Read(io.LimitReader(reader, 2), binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("proxyproto: read v2 length: %w", err)
	}
	if !header.validateLength(length) {
		return nil, fmt.Errorf("proxyproto: invalid length: %w", err)
	}

	// Return early if the length is zero, which means that
	// there's no address information and TLVs present for UNSPEC.
	if length == 0 {
		return header, nil
	}

	if _, err := reader.Peek(int(length)); err != nil {
		return nil, fmt.Errorf("proxyproto: read v2 payload: %w", err)
	}

	// Length-limited reader for payload section
	payloadReader := io.LimitReader(reader, int64(length)).(*io.LimitedReader)

	// Read addresses and ports for protocols other than UNSPEC.
	// Ignore address information for UNSPEC, and skip straight to read TLVs,
	// since the length is greater than zero.
	if header.Protocol != UNSPEC {
		if header.Protocol.IsIPv4() {
			var addr Addr4
			if err := binary.Read(payloadReader, binary.BigEndian, &addr); err != nil {
				return nil, fmt.Errorf("proxyproto: read v2 payload: %w", err)
			}
			header.RemoteAddr = newIPAddr(header.Protocol, addr.Remote[:], addr.RemotePort)
			header.LocalAddr = newIPAddr(header.Protocol, addr.Local[:], addr.LocalPort)
		} else if header.Protocol.IsIPv6() {
			var addr Addr6
			if err := binary.Read(payloadReader, binary.BigEndian, &addr); err != nil {
				return nil, fmt.Errorf("proxyproto: read v2 payload: %w", err)
			}
			header.RemoteAddr = newIPAddr(header.Protocol, addr.Remote[:], addr.RemotePort)
			header.LocalAddr = newIPAddr(header.Protocol, addr.Local[:], addr.LocalPort)
		} else if header.Protocol.IsUnix() {
			var addr AddrUnix
			if err := binary.Read(payloadReader, binary.BigEndian, &addr); err != nil {
				return nil, fmt.Errorf("proxyproto: read v2 payload: %w", err)
			}

			network := "unix"
			if header.Protocol.IsDatagram() {
				network = "unixgram"
			}

			header.RemoteAddr = &net.UnixAddr{
				Net:  network,
				Name: parseUnixName(addr.Remote[:]),
			}
			header.LocalAddr = &net.UnixAddr{
				Net:  network,
				Name: parseUnixName(addr.Local[:]),
			}
		}
	}

	// Copy bytes for optional Type-Length-Value vector
	header.RawTLVs = make([]byte, payloadReader.N) // Allocate minimum size slice
	if _, err = io.ReadFull(payloadReader, header.RawTLVs); err != nil && err != io.EOF {
		return nil, fmt.Errorf("proxyproto: read v2 payload: %w", err)
	}

	return header, nil
}

// Command represents the command in proxy protocol v2.
// Command doesn't exist in v1 but it should be set since other parts of
// this library may rely on it for determining connection details.
type Command byte

const (
	// LOCAL represents the LOCAL command in v2,
	// in which case no address information is expected.
	LOCAL Command = '\x20'
	// PROXY represents the PROXY command in v2,
	// in which case valid local/remote address and port information is expected.
	PROXY Command = '\x21'
)

var supportedCommand = map[Command]bool{
	LOCAL: true,
	PROXY: true,
}

// IsLocal returns true if the command in v2 is LOCAL or the transport in v1 is UNKNOWN,
// i.e. when no address information is expected, false otherwise.
func (pvc Command) IsLocal() bool {
	return LOCAL == pvc
}

// IsProxy returns true if the command in v2 is PROXY or the transport in v1 is not UNKNOWN,
// i.e. when valid local/remote address and port information is expected, false otherwise.
func (pvc Command) IsProxy() bool {
	return PROXY == pvc
}

// Protocol represents address family and transport protocol.
type Protocol byte

const (
	UNSPEC       Protocol = '\x00'
	TCPv4        Protocol = '\x11'
	UDPv4        Protocol = '\x12'
	TCPv6        Protocol = '\x21'
	UDPv6        Protocol = '\x22'
	UnixStream   Protocol = '\x31'
	UnixDatagram Protocol = '\x32'
)

// IsIPv4 returns true if the address family is IPv4 (AF_INET4), false otherwise.
func (ap Protocol) IsIPv4() bool {
	return ap&0xF0 == 0x10
}

// IsIPv6 returns true if the address family is IPv6 (AF_INET6), false otherwise.
func (ap Protocol) IsIPv6() bool {
	return ap&0xF0 == 0x20
}

// IsUnix returns true if the address family is UNIX (AF_UNIX), false otherwise.
func (ap Protocol) IsUnix() bool {
	return ap&0xF0 == 0x30
}

// IsStream returns true if the transport protocol is TCP or STREAM (SOCK_STREAM), false otherwise.
func (ap Protocol) IsStream() bool {
	return ap&0x0F == 0x01
}

// IsDatagram returns true if the transport protocol is UDP or DGRAM (SOCK_DGRAM), false otherwise.
func (ap Protocol) IsDatagram() bool {
	return ap&0x0F == 0x02
}

// IsUnspec returns true if the transport protocol or address family is unspecified, false otherwise.
func (ap Protocol) IsUnspec() bool {
	return (ap&0xF0 == 0x00) || (ap&0x0F == 0x00)
}

func newIPAddr(transport Protocol, ip net.IP, port uint16) net.Addr {
	if transport.IsStream() {
		return &net.TCPAddr{IP: ip, Port: int(port)}
	} else if transport.IsDatagram() {
		return &net.UDPAddr{IP: ip, Port: int(port)}
	} else {
		return nil
	}
}

func parseUnixName(b []byte) string {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return string(b)
	}
	return string(b[:i])
}
