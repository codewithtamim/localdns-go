package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/stack"
)

// Engine runs the VPN tunnel runtime backed by gVisor netstack.
type Engine struct {
	ctx      context.Context
	cancel   context.CancelFunc
	device   *stack.Device
	outbound *transport.OutboundDialer
	udpNat   *transport.UdpNat
	tun      io.Closer
}

// NewEngine creates a connected DNS relay session.
func NewEngine(
	fakedns string, resolver doh.Resolver, tun io.Closer, protector platform.Protector, listener stats.Listener,
) (*Engine, error) {
	if listener == nil {
		return nil, errors.New("listener is required")
	}

	fakeUDPAddr, err := net.ResolveUDPAddr("udp", fakedns)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve fakedns: %w", err)
	}

	tunFile, ok := tun.(*os.File)
	if !ok {
		return nil, errors.New("tun must be *os.File")
	}

	ctx, cancel := context.WithCancel(context.Background())

	outbound, err := transport.NewOutboundDialer(fakeUDPAddr.AddrPort(), resolver, protector, listener)
	if err != nil {
		cancel()
		return nil, err
	}

	udpNat, err := transport.NewUdpNat(ctx, fakeUDPAddr.AddrPort(), resolver, protector, listener)
	if err != nil {
		cancel()
		return nil, err
	}

	device, err := stack.NewDevice(tunFile, stack.DefaultMTU(), outbound, udpNat)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create netstack device: %w", err)
	}

	return &Engine{
		ctx:      ctx,
		cancel:   cancel,
		device:   device,
		outbound: outbound,
		udpNat:   udpNat,
		tun:      tun,
	}, nil
}

// SetDNS updates the active DNS resolver for all relay transports.
func (e *Engine) SetDNS(resolver doh.Resolver) {
	_ = e.outbound.SetDNS(resolver)
	_ = e.udpNat.SetDNS(resolver)
}

// Disconnect tears down the relay session.
func (e *Engine) Disconnect() {
	e.cancel()
	if e.device != nil {
		e.device.Close()
		e.device = nil
	} else if e.tun != nil {
		e.tun.Close()
	}
}
