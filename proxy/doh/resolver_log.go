package doh

import (
	"time"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/common/log"
)

// LogDNSResolverQuery writes one log line per upstream DNS query so logcat can
// distinguish DoH vs DoT (transport) and see QNAMEs. Call from each Resolver.Query
// after the query finishes.
func LogDNSResolverQuery(transport, server string, q, response []byte, before, after time.Time, err error, cancelled bool) {
	qnames, nameErr := QueryQuestionNames(q)
	latencyMs := after.Sub(before).Milliseconds()
	args := []any{
		"transport", transport,
		"server", server,
		"latencyMs", latencyMs,
		"respLen", len(response),
	}
	if nameErr != nil {
		args = append(args, "namesUnpackErr", nameErr.Error(), "qLen", len(q))
	} else {
		args = append(args, "names", qnames)
	}
	switch {
	case err != nil && cancelled:
		args = append(args, "err", "cancelled")
		log.Debug("DNS query", args...)
	case err != nil:
		args = append(args, "err", err)
		log.Warn("DNS query", args...)
	default:
		log.Info("DNS query", args...)
	}
}
