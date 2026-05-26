package transport

import "net/netip"

// isEquivalentAddrPort checks if addr1 and addr2 are equivalent. More specifically, it will treat
// "ffff::127.0.0.1" (IPv4-in-6) and "127.0.0.1" (IPv4) as equivalent, even though they are "!=" in Go.
func isEquivalentAddrPort(addr1, addr2 netip.AddrPort) bool {
	return addr1.Addr().Unmap() == addr2.Addr().Unmap() && addr1.Port() == addr2.Port()
}
