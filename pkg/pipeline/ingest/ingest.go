/*
 * Copyright (C) 2021 IBM, Inc.
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

package ingest

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	operationalMetrics "github.com/netobserv/flowlogs-pipeline/pkg/operational/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type Ingester interface {
	Ingest(out chan<- []config.GenericMap)
}
type IngesterNone struct {
}

var queueLength = operationalMetrics.NewGauge(prometheus.GaugeOpts{
	Name: "ingest_collector_queue_length",
	Help: "Queue length",
})

var linesProcessed = operationalMetrics.NewCounter(prometheus.CounterOpts{
	Name: "ingest_collector_flow_logs_processed",
	Help: "Number of log lines (flow logs) processed",
})

var packetsCount = operationalMetrics.NewCounter(prometheus.CounterOpts{
	Name: "ingest_packets_captured",
	Help: "Number of packet captured",
})

var packetsSize = operationalMetrics.NewCounter(prometheus.CounterOpts{
	Name: "ingest_packets_captured_size",
	Help: "Size of all packet captured",
})

func processPacketMetrics(record config.GenericMap) {
	if packets_raw, ok := record["Packets"]; ok {
		if packets, ok := packets_raw.(uint64); ok {
			packetsCount.Add(float64(packets))
		}
	}
	if bytes_raw, ok := record["Bytes"]; ok {
		if bytes, ok := bytes_raw.(uint64); ok {
			packetsSize.Add(float64(bytes))
		}
	}

}
