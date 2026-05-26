package stack

import (
	"fmt"
	"io"
	"net"
	"os"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const (
	nicID      = 1
	defaultMTU = 1500
)

// DefaultMTU returns the MTU used for the TUN-backed netstack.
func DefaultMTU() uint32 {
	return defaultMTU
}

// VPN addressing matches LocalDnsVpnService / LocalDnsBackend.
const (
	vpnClientIPv4 = "10.111.222.1"
	vpnPrefixLen  = 24
)

// Device runs gVisor netstack on an Android TUN fd.
type Device struct {
	stack *stack.Stack
	tun   io.Closer
}

// NewDevice attaches gVisor to tunFile using fdbased (see sample/gvisor tun_tcp_connect).
func NewDevice(
	tunFile *os.File,
	mtu uint32,
	outbound TCPDialer,
	udpNat UDPForwarder,
) (*Device, error) {
	if mtu == 0 {
		mtu = defaultMTU
	}

	s := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
	})

	linkEP, err := fdbased.New(&fdbased.Options{
		FDs:            []int{int(tunFile.Fd())},
		MTU:            mtu,
		EthernetHeader: false,
	})
	if err != nil {
		return nil, fmt.Errorf("fdbased.New: %w", err)
	}

	if terr := s.CreateNIC(nicID, linkEP); terr != nil {
		return nil, fmt.Errorf("CreateNIC: %s", terr)
	}

	clientAddr := tcpip.AddrFromSlice(net.ParseIP(vpnClientIPv4).To4())
	protocolAddr := tcpip.ProtocolAddress{
		Protocol: ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{
			Address:   clientAddr,
			PrefixLen: vpnPrefixLen,
		},
	}
	if terr := s.AddProtocolAddress(nicID, protocolAddr, stack.AddressProperties{}); terr != nil {
		return nil, fmt.Errorf("AddProtocolAddress: %s", terr)
	}

	s.SetRouteTable([]tcpip.Route{{
		Destination: header.IPv4EmptySubnet,
		NIC:         nicID,
	}})

	if terr := s.SetSpoofing(nicID, true); terr != nil {
		return nil, fmt.Errorf("SetSpoofing: %s", terr)
	}
	if terr := s.SetPromiscuousMode(nicID, true); terr != nil {
		return nil, fmt.Errorf("SetPromiscuousMode: %s", terr)
	}

	registerTCPForwarder(s, outbound)
	registerUDPForwarder(s, udpNat)

	return &Device{stack: s, tun: tunFile}, nil
}

// Close tears down the netstack and TUN handle.
func (d *Device) Close() {
	if d.stack != nil {
		d.stack.Close()
		d.stack.Wait()
		d.stack = nil
	}
	if d.tun != nil {
		d.tun.Close()
		d.tun = nil
	}
}

func registerTCPForwarder(s *stack.Stack, outbound TCPDialer) {
	fwd := tcp.NewForwarder(s, 256*1024, 65535, func(r *tcp.ForwarderRequest) {
		handleTCPForwarder(r, outbound)
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, fwd.HandlePacket)
}

func registerUDPForwarder(s *stack.Stack, udpNat UDPForwarder) {
	fwd := udp.NewForwarder(s, udpNat.HandleForwarder)
	s.SetTransportProtocolHandler(udp.ProtocolNumber, fwd.HandlePacket)
}
