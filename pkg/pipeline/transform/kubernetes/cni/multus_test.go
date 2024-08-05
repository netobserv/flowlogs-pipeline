package cni

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractNetStatusIPs(t *testing.T) {
	// Annotation not found => no error, no ip
	ip, mac, err := extractNetStatusIPsAndMACs(map[string]string{})
	require.NoError(t, err)
	require.Empty(t, ip)
	require.Empty(t, mac)

	// Annotation malformed => error, no ip
	ip, mac, err = extractNetStatusIPsAndMACs(map[string]string{
		statusAnnotation: "whatever",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot read annotation")
	require.Empty(t, ip)
	require.Empty(t, mac)

	// Valid annotation => no error, ip
	ip, mac, err = extractNetStatusIPsAndMACs(map[string]string{
		statusAnnotation: `
		[{
			"name": "cbr0",
			"ips": [
					"10.244.1.73"
			],
			"default": true,
			"dns": {}
		},{
			"name": "macvlan-conf",
			"interface": "net1",
			"ips": [
					"192.168.1.205"
			],
			"mac": "86:1d:96:ff:55:0d",
			"dns": {}
		}]
		`,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"10.244.1.73", "192.168.1.205"}, ip)
	require.Equal(t, []string{"86:1D:96:FF:55:0D"}, mac)

}
