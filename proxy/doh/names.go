package doh

import (
	"golang.org/x/net/dns/dnsmessage"
)

// QueryQuestionNames returns QNAME strings from the question section of a DNS
// wire message (UDP or TCP payload). The returned names use [dnsmessage.Name]
// string form (typically a trailing dot). Unpack errors yield a non-nil error.
func QueryQuestionNames(q []byte) ([]string, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(q); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(msg.Questions))
	for _, qu := range msg.Questions {
		out = append(out, qu.Name.String())
	}
	return out, nil
}
