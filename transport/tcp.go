package transport

import (
	"io"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"
)

func makeTCPSocketSummary(dest netip.AddrPort) *stats.TCPSocketSummary {
	summary := &stats.TCPSocketSummary{
		ServerPort: int16(dest.Port()),
	}
	if summary.ServerPort != 0 && summary.ServerPort != 80 && summary.ServerPort != 443 {
		summary.ServerPort = -1
	}
	return summary
}

type tcpWrapConn struct {
	split.DuplexConn

	wg           *sync.WaitGroup
	rDone, wDone atomic.Bool

	beginTime time.Time
	stats     *stats.TCPSocketSummary
	listener  stats.TCPListener
}

func makeTCPWrapConn(c split.DuplexConn, summary *stats.TCPSocketSummary, listener stats.TCPListener) *tcpWrapConn {
	conn := &tcpWrapConn{
		DuplexConn: c,
		wg:         &sync.WaitGroup{},
		beginTime:  time.Now(),
		stats:      summary,
		listener:   listener,
	}

	conn.wg.Add(2)
	go func() {
		conn.wg.Wait()
		conn.stats.Duration = int32(time.Since(conn.beginTime))
		if conn.listener != nil {
			conn.listener.OnTCPSocketClosed(conn.stats)
		}
	}()

	return conn
}

func (conn *tcpWrapConn) Close() error {
	defer conn.close(&conn.wDone)
	defer conn.close(&conn.rDone)
	return conn.DuplexConn.Close()
}

func (conn *tcpWrapConn) CloseRead() error {
	defer conn.close(&conn.rDone)
	if dc, ok := conn.DuplexConn.(interface{ CloseRead() error }); ok {
		return dc.CloseRead()
	}
	return nil
}

func (conn *tcpWrapConn) CloseWrite() error {
	defer conn.close(&conn.wDone)
	if dc, ok := conn.DuplexConn.(interface{ CloseWrite() error }); ok {
		return dc.CloseWrite()
	}
	return nil
}

func (conn *tcpWrapConn) Read(b []byte) (n int, err error) {
	defer func() {
		conn.stats.DownloadBytes += int64(n)
	}()
	return conn.DuplexConn.Read(b)
}

func (conn *tcpWrapConn) WriteTo(w io.Writer) (n int64, err error) {
	defer func() {
		conn.stats.DownloadBytes += n
	}()
	return io.Copy(w, conn.DuplexConn)
}

func (conn *tcpWrapConn) Write(b []byte) (n int, err error) {
	defer func() {
		conn.stats.UploadBytes += int64(n)
	}()
	return conn.DuplexConn.Write(b)
}

func (conn *tcpWrapConn) ReadFrom(r io.Reader) (n int64, err error) {
	defer func() {
		conn.stats.UploadBytes += n
	}()
	return io.Copy(conn.DuplexConn, r)
}

func (conn *tcpWrapConn) close(done *atomic.Bool) {
	if done.CompareAndSwap(false, true) {
		conn.wg.Done()
	}
}
