package socket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/log"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
)

func makeControl(p platform.Protector) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if !p.Protect(int32(fd)) {
				// TODO: Record and report these errors.
				log.Errorf("Failed to protect a %s socket", network)
			}
		})
	}
}

// Returns the first IP address that is of the desired family.
func scan(ips []string, wantV4 bool) string {
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			// `ip` failed to parse.  Skip it.
			continue
		}
		isV4 := parsed.To4() != nil
		if isV4 == wantV4 {
			return ip
		}
	}
	return ""
}

// Given a slice of IP addresses, and a transport address, return a transport
// address with the IP replaced by the first IP of the same family in `ips`, or
// by the first address of a different family if there are none of the same.
func replaceIP(addr string, ips []string) (string, error) {
	if len(ips) == 0 {
		return "", errors.New("No resolvers available")
	}
	orighost, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	origip := net.ParseIP(orighost)
	if origip == nil {
		return "", fmt.Errorf("Can't parse resolver IP: %s", orighost)
	}
	isV4 := origip.To4() != nil
	newIP := scan(ips, isV4)
	if newIP == "" {
		// There are no IPs of the desired address family.  Use a different family.
		newIP = ips[0]
	}
	return net.JoinHostPort(newIP, port), nil
}

// MakeDialer creates a new Dialer.  Recipients can safely mutate
// any public field except Control and Resolver, which are both populated.
func MakeDialer(p platform.Protector) *net.Dialer {
	if p == nil {
		return &net.Dialer{}
	}
	d := &net.Dialer{
		Control: makeControl(p),
	}
	resolverDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		resolvers := strings.Split(p.GetResolvers(), ",")
		newAddress, err := replaceIP(address, resolvers)
		if err != nil {
			return nil, err
		}
		return d.DialContext(ctx, network, newAddress)
	}
	d.Resolver = &net.Resolver{
		PreferGo: true,
		Dial:     resolverDialer,
	}
	return d
}

// MakeListenConfig returns a new ListenConfig that creates protected
// listener sockets.
func MakeListenConfig(p platform.Protector) *net.ListenConfig {
	if p == nil {
		return &net.ListenConfig{}
	}
	return &net.ListenConfig{
		Control: makeControl(p),
	}
}
