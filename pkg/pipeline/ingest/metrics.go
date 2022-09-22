package ingest

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	*operational.Metrics
	stage          string
	flowsProcessed prometheus.Counter
}

func newMetrics(opMetrics *operational.Metrics, stage string, inGaugeFunc func() int) *metrics {
	opMetrics.CreateInQueueSizeGauge(stage, inGaugeFunc)
	return &metrics{
		Metrics:        opMetrics,
		stage:          stage,
		flowsProcessed: opMetrics.CreateFlowsProcessedCounter(stage),
	}
}

func (m *metrics) createOutQueueLen(out chan<- []config.GenericMap) {
	m.CreateOutQueueSizeGauge(m.stage, func() int { return len(out) })
}
