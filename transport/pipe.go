package transport

import (
	"errors"
	"io"
	"net"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"
)

type pipeConn struct {
	net.Conn
	remote net.Conn
}

var _ split.DuplexConn = (*pipeConn)(nil)

func newPipeConn(local, remote net.Conn) (split.DuplexConn, error) {
	if local == nil || remote == nil {
		return nil, errors.New("local conn and remote conn are required")
	}
	return &pipeConn{Conn: local, remote: remote}, nil
}

func (c *pipeConn) Close() error {
	return errors.Join(c.CloseRead(), c.CloseWrite())
}

func (c *pipeConn) CloseRead() error {
	return c.remote.Close()
}

func (c *pipeConn) CloseWrite() error {
	return c.Conn.Close()
}

func (c *pipeConn) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(c.Conn, r)
}
