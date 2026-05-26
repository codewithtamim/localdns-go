package session

import (
	"context"
	"errors"
	"fmt"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
)

func probeResolver(r doh.Resolver) error {
	resp, err := r.Query(context.Background(), dnsProbeQuery)
	if err != nil {
		return fmt.Errorf("failed to send query: %w", err)
	}
	if len(resp) == 0 {
		return errors.New("invalid DNS response")
	}
	return nil
}

// ProbeDoH checks whether the DoH server responds correctly to a test query.
func ProbeDoH(s *DoHServer) error {
	if s == nil {
		return errors.New("nil DoH server")
	}
	return probeResolver(s.r)
}

// ProbeDoT checks whether the DoT server responds correctly to a test query.
func ProbeDoT(s *DoTServer) error {
	if s == nil {
		return errors.New("nil DoT server")
	}
	return probeResolver(s.r)
}
