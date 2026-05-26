package session

import (
	"errors"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/tun"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/core"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
)

// Session represents a DNS relay communication session.
type Session struct {
	*core.Engine
}

// SetDoHServer replaces the active DNS transport with the given DoH server.
func (s *Session) SetDoHServer(svr *DoHServer) {
	if svr == nil {
		return
	}
	s.SetDNS(svr.r)
}

// SetDoTServer replaces the active DNS transport with the given DoT server.
func (s *Session) SetDoTServer(svr *DoTServer) {
	if svr == nil {
		return
	}
	s.SetDNS(svr.r)
}

// ConnectSessionDoH connects a TUN session using DNS-over-HTTPS.
func ConnectSessionDoH(
	fd int, fakedns string, dns *DoHServer, protector platform.Protector, listener stats.Listener,
) (*Session, error) {
	if dns == nil {
		return nil, errors.New("dns must not be nil")
	}
	return connectSessionWithResolver(fd, fakedns, dns.r, protector, listener)
}

// ConnectSessionDoT connects a TUN session using DNS-over-TLS.
func ConnectSessionDoT(
	fd int, fakedns string, dns *DoTServer, protector platform.Protector, listener stats.Listener,
) (*Session, error) {
	if dns == nil {
		return nil, errors.New("dns must not be nil")
	}
	return connectSessionWithResolver(fd, fakedns, dns.r, protector, listener)
}

func connectSessionWithResolver(
	fd int, fakedns string, r doh.Resolver, protector platform.Protector, listener stats.Listener,
) (*Session, error) {
	tunDev, err := tun.MakeTunDeviceFromFD(fd)
	if err != nil {
		return nil, err
	}
	engine, err := core.NewEngine(fakedns, r, tunDev, protector, listener)
	if err != nil {
		tunDev.Close()
		return nil, err
	}
	return &Session{engine}, nil
}
