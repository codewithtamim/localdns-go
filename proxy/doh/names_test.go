package doh

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestQueryQuestionNames(t *testing.T) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: 0x42, Response: false})
	require.NoError(t, b.StartQuestions())
	require.NoError(t, b.Question(dnsmessage.Question{
		Name:  dnsmessage.MustNewName("example.com."),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}))
	packed, err := b.Finish()
	require.NoError(t, err)

	names, err := QueryQuestionNames(packed)
	require.NoError(t, err)
	require.Equal(t, []string{"example.com."}, names)
}

func TestQueryQuestionNames_invalid(t *testing.T) {
	_, err := QueryQuestionNames([]byte{1, 2, 3})
	require.Error(t, err)
}
