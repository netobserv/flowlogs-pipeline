/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
