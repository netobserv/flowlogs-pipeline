package operational

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestSharedCounterVecRetainsBothInputs(t *testing.T) {
	previous := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	t.Cleanup(func() { prometheus.DefaultRegisterer = previous })
	metrics := NewMetrics(&config.MetricsSettings{})
	definition := DefineMetric("shared_ingest_errors_test", "Test errors", TypeCounter, "stage")
	first := metrics.NewCounterVec(&definition)
	second := metrics.NewCounterVec(&definition)
	require.Same(t, first, second)
	first.WithLabelValues("grpc").Inc()
	second.WithLabelValues("haproxy").Inc()
	value := &dto.Metric{}
	require.NoError(t, first.WithLabelValues("haproxy").Write(value))
	require.EqualValues(t, 1, value.GetCounter().GetValue())
}
