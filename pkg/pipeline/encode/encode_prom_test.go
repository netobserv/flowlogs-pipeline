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

package encode

import (
	"math/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

const testConfig = `---
log-level: debug
pipeline:
  - name: encode1
parameters:
  - name: encode1
    encode:
      type: prom
      prom:
        prefix: test_
        expiryTime: 1s
        metrics:
          - name: Bytes
            type: gauge
            filter: {key: dstAddr, value: 10.1.2.4}
            valueKey: bytes
            labels:
              - srcAddr
              - dstAddr
              - srcPort
          - name: Packets
            type: counter
            filter: {key: dstAddr, value: 10.1.2.4}
            valueKey: packets
            labels:
              - srcAddr
              - dstAddr
              - dstPort
          - name: subnetHistogram
            type: histogram
            valueKey: aggregate
            labels:
          - name: subnetHistogram2
            type: agg_histogram
            valueKey: aggregate
            labels:
`

func initProm(params *api.PromEncode) (*EncodeProm, error) {
	// We need to re-instanciate globals used here and there, to avoid errors such as:
	//  "panic: http: multiple registrations for /metrics"
	//  TODO: remove use of default globals.
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	http.DefaultServeMux = http.NewServeMux()
	opMetrics := operational.NewMetrics(&config.MetricsSettings{})
	enc, err := NewEncodeProm(opMetrics, config.StageParam{Encode: &config.Encode{Prom: params}})
	if err != nil {
		return nil, err
	}
	prom := enc.(*EncodeProm)
	return prom, nil
}

func Test_NewEncodeProm(t *testing.T) {
	v, cfg := test.InitConfig(t, testConfig)
	require.NotNil(t, v)
	encodeProm, err := initProm(cfg.Parameters[0].Encode.Prom)
	require.NoError(t, err)

	require.Equal(t, 1, len(encodeProm.counters))
	require.Equal(t, 1, len(encodeProm.gauges))
	require.Equal(t, 1, len(encodeProm.histos))
	require.Equal(t, 1, len(encodeProm.aggHistos))
	require.Equal(t, time.Second, encodeProm.expiryTime)

	require.Equal(t, encodeProm.gauges[0].info.Name, "Bytes")
	expectedList := []string{"srcAddr", "dstAddr", "srcPort"}
	require.Equal(t, encodeProm.gauges[0].info.Labels, expectedList)

	require.Equal(t, encodeProm.counters[0].info.Name, "Packets")
	expectedList = []string{"srcAddr", "dstAddr", "dstPort"}
	require.Equal(t, encodeProm.counters[0].info.Labels, expectedList)
	entry := test.GetExtractMockEntry()
	encodeProm.Encode(entry)

	// verify entries are in cache; one for the gauge and one for the counter
	entriesMapLen := encodeProm.mCache.GetCacheLen()
	require.Equal(t, 2, entriesMapLen)

	// wait a couple seconds so that the entry will expire
	time.Sleep(2 * time.Second)
	encodeProm.mCache.CleanupExpiredEntries(encodeProm.expiryTime, encodeProm.Cleanup)
	entriesMapLen = encodeProm.mCache.GetCacheLen()
	require.Equal(t, 0, entriesMapLen)
}

func Test_CustomMetric(t *testing.T) {
	metrics := []config.GenericMap{{
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "SYN",
		"dir":     float64(0),
		"bytes":   7,
		"packets": 1,
		"latency": 0.1,
	}, {
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "RST",
		"dir":     int(0),
		"bytes":   1,
		"packets": 1,
		"latency": 0.05,
	}, {
		"srcIP":   "10.0.0.1",
		"dstIP":   "30.0.0.3",
		"flags":   "SYN",
		"dir":     1,
		"bytes":   12,
		"packets": 2,
		"latency": 0.2,
	}}
	var expirtyTimeDuration api.Duration
	expirtyTimeDuration.Duration = time.Duration(60 * time.Second)
	params := api.PromEncode{
		Prefix:     "test_",
		ExpiryTime: expirtyTimeDuration,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
		}, {
			Name:     "packets_total",
			Type:     "counter",
			ValueKey: "packets",
			Labels:   []string{"srcIP", "dstIP"},
		}, {
			Name:     "latency_seconds",
			Type:     "histogram",
			ValueKey: "latency",
			Labels:   []string{"srcIP", "dstIP"},
			Buckets:  []float64{},
		}, {
			Name:     "flows_total",
			Type:     "counter",
			ValueKey: "", // empty valuekey means it's a records counter
			Labels:   []string{"srcIP", "dstIP"},
		}, {
			Name:     "flows_incoming",
			Type:     "counter",
			Filter:   api.PromMetricsFilter{Key: "dir", Value: "0"},
			ValueKey: "", // empty valuekey means it's a records counter
		}, {
			Name:     "flows_outgoing",
			Type:     "counter",
			Filter:   api.PromMetricsFilter{Key: "dir", Value: "1"},
			ValueKey: "", // empty valuekey means it's a records counter
		}},
	}

	encodeProm, err := initProm(&params)
	require.NoError(t, err)
	for _, metric := range metrics {
		encodeProm.Encode(metric)
	}
	time.Sleep(100 * time.Millisecond)

	exposed := test.ReadExposedMetrics(t)

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
	require.Contains(t, exposed, `test_flows_total{dstIP="10.0.0.1",srcIP="20.0.0.2"} 2`)
	require.Contains(t, exposed, `test_flows_total{dstIP="30.0.0.3",srcIP="10.0.0.1"} 1`)
	require.Contains(t, exposed, `test_flows_incoming 2`)
	require.Contains(t, exposed, `test_flows_outgoing 1`)
}

func Test_MetricTTL(t *testing.T) {
	metrics := []config.GenericMap{{
		"srcIP": "20.0.0.2",
		"dstIP": "10.0.0.1",
		"bytes": 7,
	}, {
		"srcIP": "20.0.0.2",
		"dstIP": "10.0.0.1",
		"bytes": 1,
	}, {
		"srcIP": "10.0.0.1",
		"dstIP": "30.0.0.3",
		"bytes": 12,
	}}

	var expirtyTimeDuration api.Duration
	expirtyTimeDuration.Duration = time.Duration(1 * time.Second)
	params := api.PromEncode{
		Prefix:     "test_",
		ExpiryTime: expirtyTimeDuration,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
		}},
	}

	encodeProm, err := initProm(&params)
	require.NoError(t, err)

	for _, metric := range metrics {
		encodeProm.Encode(metric)
	}
	exposed := test.ReadExposedMetrics(t)

	require.Contains(t, exposed, `test_bytes_total{dstIP="10.0.0.1",srcIP="20.0.0.2"}`)
	require.Contains(t, exposed, `test_bytes_total{dstIP="30.0.0.3",srcIP="10.0.0.1"}`)

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Scrape a second time
	exposed = test.ReadExposedMetrics(t)

	require.NotContains(t, exposed, `test_bytes_total{dstIP="10.0.0.1",srcIP="20.0.0.2"}`)
	require.NotContains(t, exposed, `test_bytes_total{dstIP="30.0.0.3",srcIP="10.0.0.1"}`)
}

func buildFlow() config.GenericMap {
	return config.GenericMap{
		"srcIP":   "10.0.0." + strconv.Itoa(rand.Intn(20)),
		"dstIP":   "10.0.0." + strconv.Itoa(rand.Intn(20)),
		"flags":   "SYN",
		"bytes":   rand.Intn(100),
		"packets": rand.Intn(10),
		"latency": rand.Float64(),
	}
}

func hundredFlows() []config.GenericMap {
	flows := make([]config.GenericMap, 100)
	for i := 0; i < 100; i++ {
		flows[i] = buildFlow()
	}
	return flows
}

func BenchmarkPromEncode(b *testing.B) {
	var expirtyTimeDuration api.Duration
	expirtyTimeDuration.Duration = time.Duration(60 * time.Second)
	params := api.PromEncode{
		Prefix:     "test_",
		ExpiryTime: expirtyTimeDuration,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
		}, {
			Name:     "packets_total",
			Type:     "counter",
			ValueKey: "packets",
			Labels:   []string{"srcIP", "dstIP"},
		}, {
			Name:     "latency_seconds",
			Type:     "histogram",
			ValueKey: "latency",
			Labels:   []string{"srcIP", "dstIP"},
			Buckets:  []float64{},
		}},
	}

	prom, err := initProm(&params)
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		for _, metric := range hundredFlows() {
			prom.Encode(metric)
		}
	}
}
