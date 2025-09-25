package opentelemetry

import (
	"context"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
)

type wrappedMeter struct {
	metric.Meter
	back     metric.Meter
	counters map[string]*wrappedCounter
}

func newWrappedMeter(ctx context.Context, cfg *api.EncodeOtlpMetrics) (*wrappedMeter, error) {
	res := newResource()
	mp, err := NewOtlpMetricsProvider(ctx, cfg, res)
	if err != nil {
		return nil, err
	}
	return &wrappedMeter{back: mp.Meter("test"), counters: make(map[string]*wrappedCounter)}, nil
}

func (wm *wrappedMeter) Float64Counter(name string, options ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	c, err := wm.back.Float64Counter(name, options...)
	wc := &wrappedCounter{back: c}
	wm.counters[name] = wc
	return wc, err
}

type wrappedCounter struct {
	metric.Float64Counter
	back metric.Float64Counter
	sum  float64
}

func (wc *wrappedCounter) Add(ctx context.Context, incr float64, options ...metric.AddOption) {
	wc.sum += incr
	wc.back.Add(ctx, incr, options...)
}

func Test_EncodeOtlpMetrics(t *testing.T) {
	cfg := &api.EncodeOtlpMetrics{
		OtlpConnectionInfo: &api.OtlpConnectionInfo{
			Address:        "1.2.3.4",
			Port:           999,
			ConnectionType: "grpc",
			Headers:        nil,
		},
		Prefix: "flp_test",
		Metrics: []api.MetricsItem{
			{Name: "metric1", Type: "counter", ValueKey: "value", Labels: []string{"destination.k8s.kind", "DstSubnetLabel"}},
			{Name: "metric2", Type: "gauge", Labels: []string{"DstSubnetLabel"}},
			{Name: "metric3", Type: "counter", Labels: []string{"destination.k8s.kind", "source.k8s.kind"}},
		},
	}
	ctx := context.Background()
	wm, err := newWrappedMeter(ctx, cfg)
	require.NoError(t, err)
	encoder, err := newEncodeOtlpMetricsWithMeter(ctx, "otlp-encode", operational.NewMetrics(&config.MetricsSettings{}), cfg, wm)
	require.NoError(t, err)
	require.NotNil(t, encoder)

	assert.Len(t, wm.counters, 2)
	assert.Equal(t, float64(0), wm.counters["flp_testmetric1"].sum)
	assert.Equal(t, float64(0), wm.counters["flp_testmetric3"].sum)

	// Test empty
	encoder.Encode(config.GenericMap{})
	assert.Len(t, wm.counters, 2)
	assert.Equal(t, float64(0), wm.counters["flp_testmetric1"].sum)
	assert.Equal(t, float64(1), wm.counters["flp_testmetric3"].sum)

	encoder.Encode(config.GenericMap{"destination.k8s.kind": "foo", "DstSubnetLabel": "bar", "value": 5})
	assert.Len(t, wm.counters, 2)
	assert.Equal(t, float64(5), wm.counters["flp_testmetric1"].sum)
	assert.Equal(t, float64(2), wm.counters["flp_testmetric3"].sum)
}
