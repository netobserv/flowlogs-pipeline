package conntrack

import (
	operationalMetrics "github.com/netobserv/flowlogs-pipeline/pkg/operational/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	classificationLabel = "classification"
	typeLabel           = "type"
)

var metrics = newMetrics()

type metricsType struct {
	connStoreLength prometheus.Gauge
	inputRecords    *prometheus.CounterVec
	outputRecords   *prometheus.CounterVec
}

func newMetrics() *metricsType {
	var m metricsType

	m.connStoreLength = operationalMetrics.NewGauge(prometheus.GaugeOpts{
		Name: "conntrack_memory_connections",
		Help: "The total number of tracked connections in memory.",
	})

	m.inputRecords = operationalMetrics.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_input_records",
		Help: "The total number of input records per classification.",
	}, []string{classificationLabel})

	m.outputRecords = operationalMetrics.NewCounterVec(prometheus.CounterOpts{
		Name: "conntrack_output_records",
		Help: "The total number of output records.",
	}, []string{typeLabel})

	return &m
}
