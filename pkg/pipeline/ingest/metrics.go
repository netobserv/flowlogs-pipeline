package ingest

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	latencyHistogram = operational.DefineMetric(
		"ingest_latency_ms",
		"Latency between flow end time and ingest time, in milliseconds",
		operational.TypeHistogram,
		"stage",
	)
	flowsProcessed = operational.DefineMetric(
		"ingest_flows_processed", // This is intentionally named to emphasize its utility for flows counting, despite being a batch size distribution
		"Provides number of flows processed",
		operational.TypeCounter,
		"stage",
	)
	ingestBytes = operational.DefineMetric(
		"ingest_bytes",
		"Ingested size of the processed flows, in bytes",
		operational.TypeCounter,
		"stage",
	)
	errorsCounter = operational.DefineMetric(
		"ingest_errors",
		"Counter of errors during ingestion",
		operational.TypeCounter,
		"stage", "type", "code",
	)
)

type metrics struct {
	*operational.Metrics
	stage          string
	stageType      string
	stageDuration  prometheus.Observer
	latency        prometheus.Histogram
	flowsProcessed prometheus.Counter
	ingestBytes    prometheus.Counter
	errors         *prometheus.CounterVec
}

func newMetrics(opMetrics *operational.Metrics, stage, stageType string, inGaugeFunc func() int) *metrics {
	opMetrics.CreateInQueueSizeGauge(stage, inGaugeFunc)
	return &metrics{
		Metrics:        opMetrics,
		stage:          stage,
		stageType:      stageType,
		latency:        opMetrics.NewHistogram(&latencyHistogram, []float64{.001, .01, .1, 1, 10, 100, 1000, 10000}, stage),
		stageDuration:  opMetrics.GetOrCreateStageDurationHisto().WithLabelValues(stage),
		flowsProcessed: opMetrics.NewCounter(&flowsProcessed, stage),
		ingestBytes:    opMetrics.NewCounter(&ingestBytes, stage),
		errors:         opMetrics.NewCounterVec(&errorsCounter),
	}
}

func (m *metrics) createOutQueueLen(out chan<- config.GenericMap) {
	m.CreateOutQueueSizeGauge(m.stage, func() int { return len(out) })
}

// Increment error counter
// `code` should reflect any error code relative to this type. It can be a short string message,
// but make sure to not include any dynamic value with high cardinality
func (m *metrics) error(code string) {
	m.errors.WithLabelValues(m.stage, m.stageType, code).Inc()
}

func (m *metrics) stageDurationTimer() *operational.Timer {
	return operational.NewTimer(m.stageDuration)
}
