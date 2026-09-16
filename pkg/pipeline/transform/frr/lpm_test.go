package frr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildASNTable_LPM(t *testing.T) {
	table := BuildASNTable(map[string]uint32{
		"10.128.0.0/14":  64512,
		"10.128.0.0/24":  64513,
		"192.168.1.0/24": 65000,
	})

	asn, ok := table.LookupString("10.128.0.5")
	require.True(t, ok)
	require.Equal(t, uint32(64513), asn) // longest prefix wins

	asn, ok = table.LookupString("10.129.1.1")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)

	asn, ok = table.LookupString("192.168.1.10")
	require.True(t, ok)
	require.Equal(t, uint32(65000), asn)

	_, ok = table.LookupString("8.8.8.8")
	require.False(t, ok)
}

func TestBuildASNTable_DuplicateNormalizedCIDRs(t *testing.T) {
	table := BuildASNTable(map[string]uint32{
		"10.128.1.0/14": 64513,
		"10.128.0.0/14": 64512,
	})
	require.Equal(t, 1, table.Len())
	asn, ok := table.LookupString("10.129.0.1")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)
}

func TestBuildASNTable_SkipsASNZeroAndInvalid(t *testing.T) {
	table := BuildASNTable(map[string]uint32{
		"10.0.0.0/8":     0,
		"not-a-cidr":     64512,
		"192.168.0.0/16": 64512,
	})
	require.Equal(t, 1, table.Len())
	asn, ok := table.LookupString("192.168.1.1")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)
}
