package stats

import "org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"

// TCPSocketSummary describes a forwarded TCP flow when it closes.
type TCPSocketSummary struct {
	DownloadBytes int64
	UploadBytes   int64
	Duration      int32
	ServerPort    int16
	Synack        int32
	Retry         *split.RetryStats
}

// UDPSocketSummary describes a non-DNS UDP association when it closes.
type UDPSocketSummary struct {
	UploadBytes   int64
	DownloadBytes int64
	Duration      int32
}

// TCPListener receives TCP flow statistics.
type TCPListener interface {
	OnTCPSocketClosed(*TCPSocketSummary)
}

// UDPListener receives UDP flow statistics.
type UDPListener interface {
	OnUDPSocketClosed(*UDPSocketSummary)
}

// Listener combines TCP and UDP statistics callbacks.
type Listener interface {
	TCPListener
	UDPListener
}
