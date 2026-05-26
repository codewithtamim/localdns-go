package session

import (
	"strings"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/socket"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/proxy/doh"
)

// DoHServer represents a DNS-over-HTTPS server.
type DoHServer struct {
	r doh.Resolver
}

// NewDoHServer creates a DoHServer that connects to the specified DoH server.
//
// url is the URL of a DoH server (no template, POST-only).
//
// ipsStr is an optional comma-separated list of IP addresses for the server. It will be used when the url
// cannot be resolved to working IP addresses. (string is required cuz gomobile doesn't support []string)
//
// protector is Android's socket protector to use for all external network activity.
//
// listener will be notified after each DNS query succeeds or fails.
func NewDoHServer(
	url string, ipsStr string, protector platform.Protector, listener DoHListener,
) (*DoHServer, error) {
	ips := []string{}
	if len(ipsStr) > 0 {
		ips = strings.Split(ipsStr, ",")
	}
	dialer := socket.MakeDialer(protector)
	t, err := doh.NewResolver(url, ips, dialer, nil, makeInternalDoHListener(listener))
	if err != nil {
		return nil, err
	}
	return &DoHServer{t}, nil
}

// dnsProbeQuery is used by [Probe] for DoH and DoT connectivity checks.
var dnsProbeQuery = []byte{
	0, 0, // [0-1]   query ID
	1, 0, // [2-3]   flags, RD=1
	0, 1, // [4-5]   QDCOUNT (number of queries) = 1
	0, 0, // [6-7]   ANCOUNT (number of answers) = 0
	0, 0, // [8-9]   NSCOUNT (number of authoritative answers) = 0
	0, 0, // [10-11] ARCOUNT (number of additional records) = 0

	// Start of first query
	7, 'y', 'o', 'u', 't', 'u', 'b', 'e',
	3, 'c', 'o', 'm',
	0,    // null terminator of FQDN (DNS root)
	0, 1, // QTYPE = A
	0, 1, // QCLASS = IN (Internet)
}

////////// event listeners

// DoHQueryToken is an opaque object used to match responses to queries.
// The same DoHQueryToken that returned by [DoHListener].OnQuery be passed
// to the corresponding [DoHListener].OnResponse.
type DoHQueryToken doh.Token

// DoHListener is an event listener that receives DoH request reports.
// Application code can implement this interface to receive these reports.
type DoHListener interface {
	// OnQuery will be called when a DoH request is issued to url.
	// Application code return an arbitrary DoHQueryToken object for internal use,
	// the same object will be passed to OnResponse.
	OnQuery(url string) DoHQueryToken

	// OnResponse will be called when a DoH response has been received.
	OnResponse(DoHQueryToken, *DoHQuerySumary)
}

// DoHStatus is an integer representing the status of a DoH transaction.
type DoHStatus = int

const (
	DoHStatusComplete      DoHStatus = doh.Complete      // Transaction completed successfully
	DoHStatusSendFailed    DoHStatus = doh.SendFailed    // Failed to send query
	DoHStatusHTTPError     DoHStatus = doh.HTTPError     // Got a non-200 HTTP status
	DoHStatusBadQuery      DoHStatus = doh.BadQuery      // Malformed input
	DoHStatusBadResponse   DoHStatus = doh.BadResponse   // Response was invalid
	DoHStatusInternalError DoHStatus = doh.InternalError // This should never happen
)

// DoHQuerySumary is the summary of a DNS transaction.
// It will be reported to [DoHListener].OnResponse when it is complete.
type DoHQuerySumary struct {
	summ *doh.Summary
}

func (q DoHQuerySumary) GetQuery() []byte     { return q.summ.Query }
func (q DoHQuerySumary) GetResponse() []byte  { return q.summ.Response }
func (q DoHQuerySumary) GetServer() string    { return q.summ.Server }
func (q DoHQuerySumary) GetStatus() DoHStatus { return q.summ.Status }
func (q DoHQuerySumary) GetHTTPStatus() int   { return q.summ.HTTPStatus }
func (q DoHQuerySumary) GetLatency() float64  { return q.summ.Latency }

// dohListenerAdapter is an adapter for the internal [doh.Listener].
type dohListenerAdapter struct {
	l DoHListener
}

// makeInternalDoHListener creates a [doh.Listener] from the public [DoHListener]
// interface that will be implemented by the application code.
func makeInternalDoHListener(l DoHListener) doh.Listener {
	if l == nil {
		return nil
	}
	return &dohListenerAdapter{l}
}

func (e dohListenerAdapter) OnQuery(url string) doh.Token {
	return e.l.OnQuery(url)
}

func (e dohListenerAdapter) OnResponse(t doh.Token, s *doh.Summary) {
	e.l.OnResponse(t, &DoHQuerySumary{s})
}
