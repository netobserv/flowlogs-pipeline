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

package write

import (
	"container/list"
	"github.com/netobserv/flowlogs2metrics/pkg/testutils"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfig = `---
log-level: debug
pipeline:
  write:
    type: prom
    prom:
      port: 9103
      prefix: test_
      expirytime: 1
      metrics:
        - name: Bytes
          type: gauge
          valuekey: bytes
          labels:
            - srcAddr
            - dstAddr
            - srcPort
        - name: Packets
          type: counter
          valuekey: packets
          labels:
            - srcAddr
            - dstAddr
            - dstPort
        - name: subnetHistogram
          type: histogram
          valuekey: aggregate
          labels:
`

func initNewPromWriter(t *testing.T) Writer {
	v := test.InitConfig(t, testConfig)
	val := v.Get("pipeline.write.prom")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)

	config.Opt.PipeLine.Encode.Prom = string(b)
	newEncode, err := NewPrometheus()
	require.Equal(t, err, nil)
	return newEncode
}

func Test_NewEncodeProm(t *testing.T) {
	newWrite := initNewPromWriter(t)
	promWriter := newWrite.(*Prometheus)
	require.Equal(t, ":9103", promWriter.port)
	require.Equal(t, "test_", promWriter.prefix)
	require.Equal(t, 3, len(promWriter.metrics))
	require.Equal(t, int64(1), promWriter.expiryTime)

	metrics := promWriter.metrics
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
	promWriter.Write(input)

	entryLabels1 := make(map[string]string, 3)
	entryLabels2 := make(map[string]string, 3)
	entryLabels1["srcAddr"] = "10.1.2.3"
	entryLabels1["dstAddr"] = "10.1.2.4"
	entryLabels1["srcPort"] = "9001"
	entryLabels2["srcAddr"] = "10.1.2.3"
	entryLabels2["dstAddr"] = "10.1.2.4"
	entryLabels2["dstPort"] = "39504"
	gEntryInfo1 := entryInfo{
		value: float64(1234),
		eInfo: entrySignature{
			Name:   "test_Bytes",
			Labels: entryLabels1,
		},
	}
	gEntryInfo2 := entryInfo{
		eInfo: entrySignature{
			Name:   "test_Packets",
			Labels: entryLabels2,
		},
		value: float64(34),
	}

	testutils.Eventually(t, 5*time.Second, func(t require.TestingT) {
		promWriter.mu.Lock()
		defer promWriter.mu.Unlock()
		require.Contains(t, promWriter.mCache, generateCacheKey(&gEntryInfo1.eInfo))
		require.Contains(t, promWriter.mCache, generateCacheKey(&gEntryInfo2.eInfo))
	})
	gaugeA, err := gInfo.promGauge.GetMetricWith(entryLabels1)
	require.Equal(t, nil, err)
	bytesA := testutil.ToFloat64(gaugeA)
	require.Equal(t, gEntryInfo1.value, bytesA)

	// verify entries are in cache; one for the gauge and one for the counter
	entriesMap := promWriter.mCache
	require.Equal(t, 2, len(entriesMap))

	eInfoBytes := generateCacheKey(&gEntryInfo1.eInfo)
	promWriter.mu.Lock()
	_, found := promWriter.mCache[eInfoBytes]
	promWriter.mu.Unlock()
	require.Equal(t, true, found)

	// wait a couple seconds so that the entry will expire
	time.Sleep(2 * time.Second)
	promWriter.cleanupExpiredEntries()
	entriesMap = promWriter.mCache
	promWriter.mu.Lock()
	require.Equal(t, 0, len(entriesMap))
	promWriter.mu.Unlock()
}

func Test_EncodeAggregate(t *testing.T) {
	metrics := []config.GenericMap{{
		"name":                      "test_aggregate",
		"operation":                 "sum",
		"record_key":                "IP",
		"by":                        "[dstIP srcIP]",
		"aggregate":                 "20.0.0.2,10.0.0.1",
		"value":                     "7",
		"test_aggregate" + "_value": "7",
		"count":                     "1",
	}}

	promWriter := &Prometheus{
		port:   ":0000",
		prefix: "test_",
		metrics: map[string]metricInfo{
			"gauge": {
				input:      "test_aggregate_value",
				labelNames: []string{"by", "aggregate"},
			},
		},
		mList:  list.New(),
		mCache: map[string]*metricCacheEntry{},
	}

	promWriter.Write(metrics)

	gEntryInfo1 := entryInfo{
		eInfo: entrySignature{
			Name: "test_gauge",
			Labels: map[string]string{
				"by":        "[dstIP srcIP]",
				"aggregate": "20.0.0.2,10.0.0.1",
			},
		},
		value: float64(7),
	}

	expectedLabels := prometheus.Labels{
		"by":        "[dstIP srcIP]",
		"aggregate": "20.0.0.2,10.0.0.1",
	}
	cacheKey := generateCacheKey(&gEntryInfo1.eInfo)
	require.Contains(t, promWriter.mCache, cacheKey)
	cachedEntry := promWriter.mCache[cacheKey]
	assert.Equal(t, cacheKey, cachedEntry.key)
	assert.Equal(t, expectedLabels, cachedEntry.labels)
}
