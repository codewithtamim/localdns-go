package dot

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDNSOverTCPWriteReadRoundTrip(t *testing.T) {
	msg := []byte{0xab, 0xcd, 0x01, 0x02, 0x03}
	var buf bytes.Buffer
	require.NoError(t, dnsOverTCPWrite(&buf, msg))
	got, err := dnsOverTCPRead(&buf)
	require.NoError(t, err)
	require.Equal(t, msg, got)
}

func TestDNSOverTCPReadInvalidLength(t *testing.T) {
	// Length prefix 1 — too short for a valid DNS message per our checks.
	r := bytes.NewReader([]byte{0, 1, 0})
	_, err := dnsOverTCPRead(r)
	require.Error(t, err)
}

func TestDNSOverTCPWriteOversize(t *testing.T) {
	msg := make([]byte, 65536)
	err := dnsOverTCPWrite(io.Discard, msg)
	require.Error(t, err)
	require.True(t, errors.Is(err, errOversizeDNS))
}

func TestDNSOverTCPWriteEmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, dnsOverTCPWrite(&buf, nil))
	// Length 0: read fails rlen < 2
	_, err := dnsOverTCPRead(&buf)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid DNS response length"))
}
