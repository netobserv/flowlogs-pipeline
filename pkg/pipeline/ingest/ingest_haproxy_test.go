package ingest

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/stretchr/testify/require"
)

func TestHAProxyUDPAndShutdown(t *testing.T) {
	h, err := NewHAProxy(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Name: "proxy", Ingest: &config.Ingest{
		Type: api.HAProxyType, HAProxy: &api.IngestHAProxy{Address: "127.0.0.1:0", MaxMessageSize: 2048},
	}})
	require.NoError(t, err)
	exit := make(chan struct{})
	h.exit = exit
	out := make(chan config.GenericMap, 1)
	done := make(chan struct{})
	go func() { h.Ingest(out); close(done) }()
	address := h.conn.LocalAddr().String()
	t.Cleanup(func() {
		close(exit)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("receiver did not shut down")
			return
		}
		// Shutdown must release its socket so a subsequent pipeline can bind it.
		replacement, err := net.ListenPacket("udp", address)
		require.NoError(t, err)
		_ = replacement.Close()
	})
	conn, err := net.Dial("udp", h.conn.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()
	// Oversized and malformed messages must not prevent later valid ingestion.
	_, err = conn.Write([]byte(strings.Repeat("x", 4096)))
	require.NoError(t, err)
	_, err = conn.Write([]byte("invalid message"))
	require.NoError(t, err)
	data, err := os.ReadFile("../../proxy/testdata/openshift.log")
	require.NoError(t, err)
	_, err = conn.Write([]byte(strings.Split(string(data), "\n")[0]))
	require.NoError(t, err)
	select {
	case record := <-out:
		require.Equal(t, "proxyRequest", record["_RecordType"])
	case <-time.After(3 * time.Second):
		t.Fatal("no proxy record received")
	}
}
