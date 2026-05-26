package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/socket"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const udpIdleTimeout = 5 * time.Minute

// UdpNat handles non-DNS UDP flows and DNS-over-UDP queries for the netstack.
type UdpNat struct {
	fakeDNSAddr netip.AddrPort
	dns         atomic.Pointer[doh.Resolver]
	dialer      *net.Dialer
	listener    stats.UDPListener
	ctx         context.Context
}

func NewUdpNat(
	ctx context.Context,
	fakeDNS netip.AddrPort,
	dns doh.Resolver,
	protector platform.Protector,
	listener stats.UDPListener,
) (*UdpNat, error) {
	if dns == nil {
		return nil, errors.New("dns is required")
	}
	nat := &UdpNat{
		fakeDNSAddr: fakeDNS,
		dialer:      socket.MakeDialer(protector),
		listener:    listener,
		ctx:         ctx,
	}
	nat.dns.Store(&dns)
	return nat, nil
}

func (nat *UdpNat) SetDNS(dns doh.Resolver) error {
	if dns == nil {
		return errors.New("dns is required")
	}
	nat.dns.Store(&dns)
	return nil
}

func (nat *UdpNat) HandleForwarder(r *udp.ForwarderRequest) bool {
	id := r.ID()
	dest, ok := addrPortFromTCPUIP(id.LocalAddress, id.LocalPort)
	if !ok {
		return false
	}

	if isEquivalentAddrPort(dest, nat.fakeDNSAddr) {
		go nat.handleDNS(r)
		return true
	}

	go nat.relay(r)
	return true
}

func (nat *UdpNat) handleDNS(r *udp.ForwarderRequest) {
	var wq waiter.Queue
	ep, terr := r.CreateEndpoint(&wq)
	if terr != nil {
		return
	}
	defer ep.Close()

	conn := gonet.NewUDPConn(&wq, ep)
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return
	}

	resp, err := (*nat.dns.Load()).Query(nat.ctx, buf[:n])
	if err != nil || len(resp) == 0 {
		return
	}
	_, _ = conn.Write(resp)
}

func (nat *UdpNat) relay(r *udp.ForwarderRequest) {
	tracker := makeTracker()
	defer func() {
		if nat.listener != nil {
			nat.listener.OnUDPSocketClosed(&stats.UDPSocketSummary{
				Duration:      int32(time.Since(tracker.start)),
				UploadBytes:   tracker.upload.Load(),
				DownloadBytes: tracker.download.Load(),
			})
		}
	}()

	id := r.ID()
	dest, ok := addrPortFromTCPUIP(id.LocalAddress, id.LocalPort)
	if !ok {
		return
	}

	var wq waiter.Queue
	ep, terr := r.CreateEndpoint(&wq)
	if terr != nil {
		return
	}
	defer ep.Close()

	stackConn := gonet.NewUDPConn(&wq, ep)
	defer stackConn.Close()

	remoteConn, err := nat.dialer.Dial("udp", dest.String())
	if err != nil {
		return
	}
	defer remoteConn.Close()

	deadline := time.Now().Add(udpIdleTimeout)
	_ = remoteConn.SetDeadline(deadline)
	_ = stackConn.SetDeadline(deadline)

	done := make(chan struct{}, 2)
	go copyUDPCount(stackConn, remoteConn, &tracker.download, done)
	go copyUDPCount(remoteConn, stackConn, &tracker.upload, done)
	<-done
}

func copyUDPCount(dst io.Writer, src io.Reader, counter *atomic.Int64, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	buf := make([]byte, 2048)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			counter.Add(int64(n))
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
