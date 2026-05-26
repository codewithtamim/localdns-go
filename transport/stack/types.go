package stack

import (
	"context"
	"net/netip"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"

	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// TCPDialer dials outbound TCP connections for forwarded flows.
type TCPDialer interface {
	DialTCP(ctx context.Context, dest netip.AddrPort) (split.DuplexConn, error)
}

// UDPForwarder handles UDP flows intercepted by the netstack.
type UDPForwarder interface {
	HandleForwarder(r *udp.ForwarderRequest) bool
}
