// Package dot implements DNS-over-TLS (DoT, RFC 7858) as a [doh.Resolver].
package dot

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/log"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh/ipmap"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/transport/split"
)

const (
	defaultDoTPort = 853
	// Match doh.hangoverDuration for consistent behavior.
	hangoverDuration    = 10 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
	readTimeout         = 20 * time.Second
)

// queryError mirrors doh.queryError status codes.
type queryError struct {
	status int
	err    error
}

func (e *queryError) Error() string { return e.err.Error() }
func (e *queryError) Unwrap() error { return e.err }

type resolver struct {
	url                string
	hostname           string
	port               int
	ips                ipmap.IPMap
	dialer             *net.Dialer
	listener           doh.Listener
	tlsConfig          *tls.Config
	hangoverLock       sync.RWMutex
	hangoverExpiration time.Time
}

// NewResolver returns a [doh.Resolver] that speaks DNS-over-TLS (RFC 7858).
//
// rawurl must use scheme "dot" or "tls", e.g. dot://dns.google or tls://1.1.1.1:853.
//
// addrs is an optional list of bootstrap addresses (IPs or hostnames) when DNS resolution fails,
// matching [doh.NewResolver].
func NewResolver(rawurl string, addrs []string, dialer *net.Dialer, listener doh.Listener) (doh.Resolver, error) {
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	parsed, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	scheme := parsed.Scheme
	if scheme != "dot" && scheme != "tls" {
		return nil, fmt.Errorf("dot: bad scheme: %s", scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("dot: empty host")
	}
	port := defaultDoTPort
	if p := parsed.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("dot: bad port: %w", err)
		}
	}

	canonical := fmt.Sprintf("dot://%s", net.JoinHostPort(host, strconv.Itoa(port)))

	t := &resolver{
		url:      canonical,
		hostname: host,
		port:     port,
		listener: listener,
		dialer:   dialer,
		ips:      ipmap.NewIPMap(dialer.Resolver),
		tlsConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			ServerName:         host,
			ClientSessionCache: tls.NewLRUClientSessionCache(64),
		},
	}
	ips := t.ips.Get(t.hostname)
	for _, addr := range addrs {
		ips.Add(addr)
	}
	if ips.Empty() {
		return nil, fmt.Errorf("dot: no IP addresses for %s", t.hostname)
	}
	return t, nil
}

func (r *resolver) GetURL() string {
	return r.url
}

func (r *resolver) dialPlainTCP(ctx context.Context, domain string, port int) (net.Conn, error) {
	log.Debug("DoT(resolver.dialPlainTCP) - dialing", "domain", domain, "port", port)
	tcpaddr := func(ip net.IP) *net.TCPAddr {
		return &net.TCPAddr{IP: ip, Port: port}
	}
	var conn net.Conn
	var err error
	ips := r.ips.Get(domain)
	confirmed := ips.Confirmed()
	if confirmed != nil {
		log.Debug("DoT(resolver.dialPlainTCP) - trying confirmed IP", "ip", confirmed)
		if conn, err = split.DialWithSplitRetry(ctx, r.dialer, tcpaddr(confirmed), nil); err == nil {
			log.Info("DoT(resolver.dialPlainTCP) - confirmed IP worked", "ip", confirmed)
			return conn, nil
		}
		log.Debug("DoT(resolver.dialPlainTCP) - confirmed IP failed", "ip", confirmed, "err", err)
		ips.Disconfirm(confirmed)
	}
	for _, ip := range ips.GetAll() {
		if ip.Equal(confirmed) {
			continue
		}
		if conn, err = split.DialWithSplitRetry(ctx, r.dialer, tcpaddr(ip), nil); err == nil {
			log.Info("DoT(resolver.dialPlainTCP) - found working IP", "ip", ip)
			return conn, nil
		}
	}
	return nil, err
}

func (r *resolver) doQuery(ctx context.Context, q []byte) (response []byte, server *net.TCPAddr, qerr *queryError) {
	if len(q) < 2 {
		qerr = &queryError{doh.BadQuery, fmt.Errorf("query length is %d", len(q))}
		return
	}

	r.hangoverLock.RLock()
	inHangover := time.Now().Before(r.hangoverExpiration)
	r.hangoverLock.RUnlock()
	if inHangover {
		response = dohTryServfail(q)
		qerr = &queryError{doh.HTTPError, errors.New("forwarder is in servfail hangover")}
		return
	}

	q, err := doh.AddEdnsPadding(q)
	if err != nil {
		qerr = &queryError{doh.InternalError, err}
		return
	}

	id := binary.BigEndian.Uint16(q)
	binary.BigEndian.PutUint16(q, 0)

	plain, err := r.dialPlainTCP(ctx, r.hostname, r.port)
	if err != nil {
		qerr = &queryError{doh.SendFailed, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}

	tlsConn := tls.Client(plain, r.tlsConfig)
	deadline := time.Now().Add(tlsHandshakeTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := tlsConn.SetDeadline(deadline); err != nil {
		_ = tlsConn.Close()
		qerr = &queryError{doh.SendFailed, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		qerr = &queryError{doh.SendFailed, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}

	if ra, ok := plain.RemoteAddr().(*net.TCPAddr); ok {
		server = ra
	}

	if err := tlsConn.SetWriteDeadline(time.Now().Add(readTimeout)); err != nil {
		_ = tlsConn.Close()
		qerr = &queryError{doh.SendFailed, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}
	if err := dnsOverTCPWrite(tlsConn, q); err != nil {
		_ = tlsConn.Close()
		st := doh.SendFailed
		if errors.Is(err, errOversizeDNS) {
			st = doh.BadQuery
		}
		qerr = &queryError{st, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}

	if err := tlsConn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		_ = tlsConn.Close()
		qerr = &queryError{doh.SendFailed, err}
		binary.BigEndian.PutUint16(q, id)
		return
	}
	resp, readErr := dnsOverTCPRead(tlsConn)
	if readErr != nil {
		_ = tlsConn.Close()
		qerr = &queryError{doh.BadResponse, readErr}
		binary.BigEndian.PutUint16(q, id)
		return
	}
	response = resp
	_ = tlsConn.Close()

	binary.BigEndian.PutUint16(q, id)
	if qerr == nil {
		if len(response) >= 2 {
			if binary.BigEndian.Uint16(response) == 0 {
				binary.BigEndian.PutUint16(response, id)
			} else {
				qerr = &queryError{doh.BadResponse, errors.New("nonzero response ID")}
			}
		} else {
			qerr = &queryError{doh.BadResponse, fmt.Errorf("response length is %d", len(response))}
		}
	}

	if qerr != nil {
		if qerr.status != doh.SendFailed {
			r.hangoverLock.Lock()
			r.hangoverExpiration = time.Now().Add(hangoverDuration)
			r.hangoverLock.Unlock()
		}
		response = dohTryServfail(q)
	} else if server != nil {
		r.ips.Get(r.hostname).Confirm(server.IP)
	}
	return
}

var errOversizeDNS = errors.New("DNS message exceeds uint16 maximum length")

func dnsOverTCPWrite(w io.Writer, msg []byte) error {
	qlen := len(msg)
	if qlen > math.MaxUint16 {
		return errOversizeDNS
	}
	buf := make([]byte, qlen+2)
	binary.BigEndian.PutUint16(buf, uint16(qlen))
	copy(buf[2:], msg)
	_, err := w.Write(buf)
	return err
}

func dnsOverTCPRead(r io.Reader) ([]byte, error) {
	var rlenBuf [2]byte
	if _, err := io.ReadFull(r, rlenBuf[:]); err != nil {
		return nil, err
	}
	rlen := int(binary.BigEndian.Uint16(rlenBuf[:]))
	if rlen < 2 || rlen > math.MaxUint16 {
		return nil, fmt.Errorf("invalid DNS response length %d", rlen)
	}
	out := make([]byte, rlen)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func dohTryServfail(q []byte) []byte {
	response, err := doh.Servfail(q)
	if err != nil {
		log.Warn("DoT(SERVFAIL) - failed to construct response", "err", err)
	}
	return response
}

func (r *resolver) Query(ctx context.Context, q []byte) ([]byte, error) {
	var token doh.Token
	if r.listener != nil {
		token = r.listener.OnQuery(r.url)
	}
	before := time.Now()
	response, server, qerr := r.doQuery(ctx, q)
	after := time.Now()

	errIsCancel := false
	var err error
	status := doh.Complete
	httpStatus := 0
	if qerr != nil {
		err = qerr
		status = qerr.status
		errIsCancel = errors.Is(qerr, context.Canceled)
	}

	if r.listener != nil && !errIsCancel {
		latency := after.Sub(before)
		var ip string
		if server != nil {
			ip = server.IP.String()
		}
		r.listener.OnResponse(token, &doh.Summary{
			Latency:    latency.Seconds(),
			Query:      q,
			Response:   response,
			Server:     ip,
			Status:     status,
			HTTPStatus: httpStatus,
		})
	}
	doh.LogDNSResolverQuery("DoT", r.url, q, response, before, after, err, errIsCancel)
	return response, err
}
