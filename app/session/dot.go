package session

import (
	"strings"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/socket"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/dot"
)

// DoTServer represents a DNS-over-TLS server.
type DoTServer struct {
	r doh.Resolver
}

// NewDoTServer creates a DoTServer that connects to the specified DoT endpoint.
//
// url uses scheme "dot" or "tls", e.g. dot://dns.google or tls://1.1.1.1:853.
//
// ipsStr is an optional comma-separated list of IP addresses used when the hostname
// cannot be resolved to working addresses.
func NewDoTServer(
	url string, ipsStr string, protector platform.Protector, listener DoHListener,
) (*DoTServer, error) {
	ips := []string{}
	if len(ipsStr) > 0 {
		ips = strings.Split(ipsStr, ",")
	}
	dialer := socket.MakeDialer(protector)
	t, err := dot.NewResolver(url, ips, dialer, makeInternalDoHListener(listener))
	if err != nil {
		return nil, err
	}
	return &DoTServer{t}, nil
}
