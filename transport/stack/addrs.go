package stack

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
)

func addrPortFromTCPUIP(addr tcpip.Address, port uint16) (netip.AddrPort, bool) {
	b := addr.AsSlice()
	if len(b) == 0 {
		return netip.AddrPort{}, false
	}
	var nip netip.Addr
	if len(b) == 4 {
		nip = netip.AddrFrom4([4]byte(b))
	} else if len(b) == 16 {
		nip = netip.AddrFrom16([16]byte(b))
	} else {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(nip, port), true
}
