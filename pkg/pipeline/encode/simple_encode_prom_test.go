package encode

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func Test_CustomMetricSimple(t *testing.T) {
	metrics := []config.GenericMap{{
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "SYN",
		"bytes":   7,
		"packets": 1,
		"latency": 0.1,
	}, {
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "RST",
		"bytes":   1,
		"packets": 1,
		"latency": 0.05,
	}, {
		"srcIP":   "10.0.0.1",
		"dstIP":   "30.0.0.3",
		"flags":   "SYN",
		"bytes":   12,
		"packets": 2,
		"latency": 0.2,
	}}

	params := api.SimplePromEncode{
		Port:       9090,
		Prefix:     "test_",
		ExpiryTime: 60,
		Metrics: []api.SimplePromMetricsItem{{
			Name:      "bytes_total",
			Type:      "counter",
			RecordKey: "bytes",
			Labels:    []string{"srcIP", "dstIP"},
		}, {
			Name:      "packets_total",
			Type:      "counter",
			RecordKey: "packets",
			Labels:    []string{"srcIP", "dstIP"},
		}, {
			Name:      "latency_seconds",
			Type:      "histogram",
			RecordKey: "latency",
			Labels:    []string{"srcIP", "dstIP"},
			Buckets:   []float64{},
		}},
	}

	newEncode, err := NewEncodeSimpleProm(config.StageParam{Encode: &config.Encode{SimpleProm: &params}})
	require.Equal(t, err, nil)

	newEncode.Encode(metrics)
	time.Sleep(500 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9090", nil)
	w := httptest.NewRecorder()

	promhttp.Handler().ServeHTTP(w, req)
	exposed := w.Body.String()

	require.Contains(t, exposed, `test_bytes_total{dstIP="10.0.0.1",srcIP="20.0.0.2"} 8`)
	require.Contains(t, exposed, `test_bytes_total{dstIP="30.0.0.3",srcIP="10.0.0.1"} 12`)
	require.Contains(t, exposed, `test_packets_total{dstIP="10.0.0.1",srcIP="20.0.0.2"} 2`)
	require.Contains(t, exposed, `test_packets_total{dstIP="30.0.0.3",srcIP="10.0.0.1"} 2`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.025"} 0`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.05"} 1`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.1"} 2`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="+Inf"} 2`)
	require.Contains(t, exposed, `test_latency_seconds_sum{dstIP="10.0.0.1",srcIP="20.0.0.2"} 0.15`)
	require.Contains(t, exposed, `test_latency_seconds_count{dstIP="10.0.0.1",srcIP="20.0.0.2"} 2`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="0.1"} 0`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="0.25"} 1`)
	require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="+Inf"} 1`)
	require.Contains(t, exposed, `test_latency_seconds_sum{dstIP="30.0.0.3",srcIP="10.0.0.1"} 0.2`)
	require.Contains(t, exposed, `test_latency_seconds_count{dstIP="30.0.0.3",srcIP="10.0.0.1"} 1`)
}
