package proxyproto

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToAddr(t *testing.T) {
	t.Run("only_tcp_udp_work", func(t *testing.T) {
		addr := toAddr(0, net.IPv4zero, 0)
		require.Nil(t, addr)
	})
}
