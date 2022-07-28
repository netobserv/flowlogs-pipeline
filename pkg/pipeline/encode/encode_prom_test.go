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
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
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
        port: 9103
        prefix: test_
        expiryTime: 1
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
`

func initNewEncodeProm(t *testing.T) Encoder {
	v, cfg := test.InitConfig(t, testConfig)
	require.NotNil(t, v)

	newEncode, err := NewEncodeProm(cfg.Parameters[0])
	require.Equal(t, err, nil)
	return newEncode
}

func Test_NewEncodeProm(t *testing.T) {
	newEncode := initNewEncodeProm(t)
	encodeProm := newEncode.(*EncodeProm)
	require.Equal(t, ":9103", encodeProm.port)
	require.Equal(t, "test_", encodeProm.prefix)
	require.Equal(t, 3, len(encodeProm.metrics))
	require.Equal(t, int64(1), encodeProm.expiryTime)
	require.Equal(t, (*api.PromTLSConf)(nil), encodeProm.tlsConfig)

	metrics := encodeProm.metrics
	assert.Contains(t, metrics, "Bytes")
	gInfo := metrics["Bytes"]
	require.Equal(t, gInfo.input, "bytes")
	expectedList := []string{"srcAddr", "dstAddr", "srcPort"}
	require.Equal(t, gInfo.labelNames, expectedList)

	assert.Contains(t, metrics, "Packets")
	cInfo := metrics["Packets"]
	require.Equal(t, cInfo.input, "packets")
	expectedList = []string{"srcAddr", "dstAddr", "dstPort"}
	require.Equal(t, cInfo.labelNames, expectedList)
	entry := test.GetExtractMockEntry()
	input := []config.GenericMap{entry}
	encodeProm.Encode(input)

	entryLabels1 := make(map[string]string, 3)
	entryLabels2 := make(map[string]string, 3)
	entryLabels1["srcAddr"] = "10.1.2.3"
	entryLabels1["dstAddr"] = "10.1.2.4"
	entryLabels1["srcPort"] = "9001"
	entryLabels2["srcAddr"] = "10.1.2.3"
	entryLabels2["dstAddr"] = "10.1.2.4"
	entryLabels2["dstPort"] = "39504"
	gEntryInfo1 := config.GenericMap{
		"Name":   "test_Bytes",
		"Labels": entryLabels1,
		"value":  1234,
	}
	gEntryInfo2 := config.GenericMap{
		"Name":   "test_Packets",
		"Labels": entryLabels2,
		"value":  34,
	}
	require.Contains(t, encodeProm.PrevRecords, gEntryInfo1)
	require.Contains(t, encodeProm.PrevRecords, gEntryInfo2)
	gaugeA, err := gInfo.promGauge.GetMetricWith(entryLabels1)
	require.Equal(t, nil, err)
	bytesA := testutil.ToFloat64(gaugeA)
	require.Equal(t, gEntryInfo1["value"], int(bytesA))

	// verify entries are in cache; one for the gauge and one for the counter
	entriesMapLen := encodeProm.mCache.GetCacheLen()
	require.Equal(t, 2, entriesMapLen)

	eInfo := entrySignature{
		Name:   "test_Bytes",
		Labels: entryLabels1,
	}

	eInfoBytes := generateCacheKey(&eInfo)
	_, found := encodeProm.mCache.GetCacheEntry(string(eInfoBytes))
	require.Equal(t, true, found)

	// wait a couple seconds so that the entry will expire
	time.Sleep(2 * time.Second)
	encodeProm.mCache.CleanupExpiredEntries(encodeProm.expiryTime, encodeProm)
	entriesMapLen = encodeProm.mCache.GetCacheLen()
	require.Equal(t, 0, entriesMapLen)
}

func Test_EncodeAggregate(t *testing.T) {
	metrics := []config.GenericMap{{
		"name":       "test_aggregate",
		"operation":  "sum",
		"record_key": "IP",
		"by":         "[dstIP srcIP]",
		"aggregate":  "20.0.0.2,10.0.0.1",
		"value":      7.0,
		"count":      1,
	}}

	newEncode := &EncodeProm{
		port:   ":0000",
		prefix: "test_",
		metrics: map[string]metricInfo{
			"gauge": {
				input: "value",
				filter: keyValuePair{
					key:   "name",
					value: "test_aggregate",
				},
				labelNames: []string{"by", "aggregate"},
			},
		},
		mCache: utils.NewTimedCache(),
	}

	newEncode.Encode(metrics)

	gEntryInfo1 := config.GenericMap{
		"Name": "test_gauge",
		"Labels": map[string]string{
			"by":        "[dstIP srcIP]",
			"aggregate": "20.0.0.2,10.0.0.1",
		},
		"value": float64(7),
	}

	expectedOutput := []config.GenericMap{gEntryInfo1}
	require.Equal(t, expectedOutput, newEncode.PrevRecords)
}

func Test_CustomMetric(t *testing.T) {
	metrics := []config.GenericMap{{
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "SYN",
		"bytes":   7,
		"packets": 1,
		"latency": 0.1,
		"hack":    "hack",
	}, {
		"srcIP":   "20.0.0.2",
		"dstIP":   "10.0.0.1",
		"flags":   "RST",
		"bytes":   1,
		"packets": 1,
		"latency": 0.05,
		"hack":    "hack",
	}, {
		"srcIP":   "10.0.0.1",
		"dstIP":   "30.0.0.3",
		"flags":   "SYN",
		"bytes":   12,
		"packets": 2,
		"latency": 0.2,
		"hack":    "hack",
	}}

	params := api.PromEncode{
		Port:       9090,
		Prefix:     "test_",
		ExpiryTime: 60,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
			Filter: api.PromMetricsFilter{
				Key:   "hack",
				Value: "hack",
			},
		}, {
			Name:     "packets_total",
			Type:     "counter",
			ValueKey: "packets",
			Labels:   []string{"srcIP", "dstIP"},
			Filter: api.PromMetricsFilter{
				Key:   "hack",
				Value: "hack",
			},
			// }, {
			// 	Name:     "latency_seconds",
			// 	Type:     "histogram",
			// 	ValueKey: "latency",
			// 	Labels:   []string{"srcIP", "dstIP"},
			// 	Filter: api.PromMetricsFilter{
			// 		Key:   "hack",
			// 		Value: "hack",
			// 	},
			// 	Buckets: []float64{},
		}},
	}

	newEncode, err := NewEncodeProm(config.StageParam{Encode: &config.Encode{Prom: &params}})
	require.Equal(t, err, nil)

	newEncode.Encode(metrics)
	time.Sleep(500 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9090", nil)
	w := httptest.NewRecorder()

	promhttp.Handler().ServeHTTP(w, req)
	exposed := w.Body.String()

	fmt.Println(exposed)

	require.Contains(t, exposed, `test_bytes_total{dstIP="10.0.0.1",srcIP="20.0.0.2"} 8`)
	require.Contains(t, exposed, `test_bytes_total{dstIP="30.0.0.3",srcIP="10.0.0.1"} 12`)
	require.Contains(t, exposed, `test_packets_total{dstIP="10.0.0.1",srcIP="20.0.0.2"} 2`)
	require.Contains(t, exposed, `test_packets_total{dstIP="30.0.0.3",srcIP="10.0.0.1"} 2`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.025"} 0`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.05"} 1`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="0.1"} 2`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="10.0.0.1",srcIP="20.0.0.2",le="+Inf"} 2`)
	// require.Contains(t, exposed, `test_latency_seconds_sum{dstIP="10.0.0.1",srcIP="20.0.0.2"} 0.15`)
	// require.Contains(t, exposed, `test_latency_seconds_count{dstIP="10.0.0.1",srcIP="20.0.0.2"} 2`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="0.1"} 0`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="0.25"} 1`)
	// require.Contains(t, exposed, `test_latency_seconds_bucket{dstIP="30.0.0.3",srcIP="10.0.0.1",le="+Inf"} 1`)
	// require.Contains(t, exposed, `test_latency_seconds_sum{dstIP="30.0.0.3",srcIP="10.0.0.1"} 0.2`)
	// require.Contains(t, exposed, `test_latency_seconds_count{dstIP="30.0.0.3",srcIP="10.0.0.1"} 1`)
}

func Test_MetricTTL(t *testing.T) {
	metrics := []config.GenericMap{{
		"srcIP": "20.0.0.2",
		"dstIP": "10.0.0.1",
		"bytes": 7,
		"hack":  "hack",
	}, {
		"srcIP": "20.0.0.2",
		"dstIP": "10.0.0.1",
		"bytes": 1,
		"hack":  "hack",
	}, {
		"srcIP": "10.0.0.1",
		"dstIP": "30.0.0.3",
		"bytes": 12,
		"hack":  "hack",
	}}

	params := api.PromEncode{
		Port:       9090,
		Prefix:     "test_",
		ExpiryTime: 1,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
			Filter: api.PromMetricsFilter{
				Key:   "hack",
				Value: "hack",
			},
		}},
	}

	newEncode, err := NewEncodeProm(config.StageParam{Encode: &config.Encode{Prom: &params}})
	require.Equal(t, err, nil)

	newEncode.Encode(metrics)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9090", nil)
	w := httptest.NewRecorder()

	promhttp.Handler().ServeHTTP(w, req)
	exposed := w.Body.String()

	require.Contains(t, exposed, `test_bytes_total{dstIP="10.0.0.1",srcIP="20.0.0.2"}`)
	require.Contains(t, exposed, `test_bytes_total{dstIP="30.0.0.3",srcIP="10.0.0.1"}`)

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Scrape a second time
	w = httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(w, req)
	exposed = w.Body.String()

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
		"hack":    "hack",
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
	params := api.PromEncode{
		Port:       9090,
		Prefix:     "test_",
		ExpiryTime: 60,
		Metrics: []api.PromMetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "bytes",
			Labels:   []string{"srcIP", "dstIP"},
			Filter: api.PromMetricsFilter{
				Key:   "hack",
				Value: "hack",
			},
		}, {
			Name:     "packets_total",
			Type:     "counter",
			ValueKey: "packets",
			Labels:   []string{"srcIP", "dstIP"},
			Filter: api.PromMetricsFilter{
				Key:   "hack",
				Value: "hack",
			},
			// }, {
			// 	Name:     "latency_seconds",
			// 	Type:     "histogram",
			// 	ValueKey: "latency",
			// 	Labels:   []string{"srcIP", "dstIP"},
			// 	Filter: api.PromMetricsFilter{
			// 		Key:   "hack",
			// 		Value: "hack",
			// 	},
			// 	Buckets: []float64{},
		}},
	}
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	http.DefaultServeMux = http.NewServeMux()
	enc, err := NewEncodeProm(config.StageParam{Encode: &config.Encode{Prom: &params}})
	if err != nil {
		b.Fatal(err)
	}
	prom := enc.(*EncodeProm)
	for i := 0; i < b.N; i++ {
		prom.Encode(hundredFlows())
	}

	err = prom.closeServer(context.Background())
	if err != nil {
		b.Fatal(err)
	}
}
