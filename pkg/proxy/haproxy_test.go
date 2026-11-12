package proxy

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenShiftLogs(t *testing.T) {
	// Captured from HTTP, edge and reencrypt routes on OpenShift 4.22.20,
	// independently matched against the HTTP server's peer socket.
	data, err := os.ReadFile("testdata/openshift.log")
	require.NoError(t, err)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		record, err := DecodeHAProxy([]byte(line), time.Unix(1800000000, 0))
		require.NoError(t, err)
		require.Equal(t, RecordType, record["_RecordType"])
		require.Equal(t, "10.129.0.34", record["ProxyClientAddr"])
		require.Equal(t, "10.129.0.32", record["SrcAddr"])
		require.Equal(t, "10.129.0.31", record["DstAddr"])
		require.Greater(t, record["SrcPort"].(int), 0)
		require.GreaterOrEqual(t, record["ProxyEndMs"].(int64), record["ProxyStartMs"].(int64))
		require.NotContains(t, record, "Packets")
		require.NotContains(t, record, "Bytes")
	}
}

func TestDecodeHAProxyValidation(t *testing.T) {
	data, err := os.ReadFile("testdata/openshift.log")
	require.NoError(t, err)
	line := strings.Split(strings.TrimSpace(string(data)), "\n")[0]
	_, suffix, found := strings.Cut(line, HAProxyPrefix)
	require.True(t, found)
	payload := HAProxyPrefix + suffix
	for _, tc := range []struct {
		name, input string
		valid       bool
		want        error
	}{
		{"raw", payload, true, nil},
		{"ipv6", strings.ReplaceAll(payload, "10.129.0.34", "2001:db8::1"), true, nil},
		{"unknown version", strings.ReplaceAll(payload, "proxy-v1", "proxy-v2"), false, ErrUnsupported},
		{"embedded in request", "GET /" + payload, false, ErrUnsupported},
		{"no backend", strings.ReplaceAll(payload, `"backend_addr":"10.129.0.32"`, `"backend_addr":"-"`), false, ErrNoBackend},
		{"bad IP", strings.ReplaceAll(payload, "10.129.0.34", "not-an-ip"), false, nil},
		{"bad port", strings.ReplaceAll(payload, `"server_port":"8080"`, `"server_port":"65536"`), false, nil},
		{"early log", strings.ReplaceAll(payload, `"duration_ms":"`, `"duration_ms":"+`), false, ErrIncomplete},
		{"truncated", payload[:len(payload)-1], false, nil},
		{"trailing payload", payload + " {}", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeHAProxy([]byte(tc.input), time.Now())
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}
