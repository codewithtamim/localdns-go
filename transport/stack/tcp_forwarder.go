package stack

import (
	"context"
	"io"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func handleTCPForwarder(r *tcp.ForwarderRequest, outbound TCPDialer) {
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
	r.Complete(false)

	stackConn := gonet.NewTCPConn(&wq, ep)
	defer stackConn.Close()

	outConn, err := outbound.DialTCP(context.Background(), dest)
	if err != nil {
		return
	}
	defer outConn.Close()

	done := make(chan struct{}, 2)
	go relayTCP(outConn, stackConn, done)
	go relayTCP(stackConn, outConn, done)
	<-done
}

func relayTCP(dst, src io.ReadWriteCloser, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	_, _ = io.Copy(dst, src)
	if dc, ok := dst.(split.DuplexConn); ok {
		_ = dc.CloseWrite()
	}
	if dc, ok := src.(split.DuplexConn); ok {
		_ = dc.CloseRead()
	}
}
