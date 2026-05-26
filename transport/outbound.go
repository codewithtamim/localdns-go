package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/socket"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"
)

// OutboundDialer dials TCP connections on behalf of the netstack forwarder.
type OutboundDialer struct {
	fakeDNSAddr netip.AddrPort
	dns         atomic.Pointer[doh.Resolver]
	dialer      *net.Dialer
	listener    stats.TCPListener
}

func NewOutboundDialer(
	fakeDNS netip.AddrPort,
	dns doh.Resolver,
	protector platform.Protector,
	listener stats.TCPListener,
) (*OutboundDialer, error) {
	if dns == nil {
		return nil, errors.New("dns is required")
	}
	od := &OutboundDialer{
		fakeDNSAddr: fakeDNS,
		dialer:      socket.MakeDialer(protector),
		listener:    listener,
	}
	od.dns.Store(&dns)
	return od, nil
}

func (od *OutboundDialer) SetDNS(dns doh.Resolver) error {
	if dns == nil {
		return errors.New("dns is required")
	}
	od.dns.Store(&dns)
	return nil
}

func (od *OutboundDialer) DialTCP(ctx context.Context, dest netip.AddrPort) (split.DuplexConn, error) {
	if isEquivalentAddrPort(dest, od.fakeDNSAddr) {
		src, dst := net.Pipe()
		go doh.Accept(*od.dns.Load(), dst)
		return newPipeConn(src, dst)
	}

	summary := makeTCPSocketSummary(dest)
	beforeConn := time.Now()
	conn, err := od.dial(ctx, dest, summary)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to target: %w", err)
	}
	summary.Synack = int32(time.Since(beforeConn).Milliseconds())
	return makeTCPWrapConn(conn, summary, od.listener), nil
}

func (od *OutboundDialer) dial(ctx context.Context, dest netip.AddrPort, summary *stats.TCPSocketSummary) (split.DuplexConn, error) {
	if dest.Port() == 443 {
		summary.Retry = &split.RetryStats{}
		return split.DialWithSplitRetry(ctx, od.dialer, net.TCPAddrFromAddrPort(dest), summary.Retry)
	}
	conn, err := od.dialer.DialContext(ctx, "tcp", dest.String())
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, errors.New("expected tcp connection")
	}
	return tcpConn, nil
}
