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
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
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
	v := test.InitConfig(t, testConfig)
	require.NotNil(t, v)

	newEncode, err := NewEncodeProm(config.Parameters[0])
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
	entriesMap := encodeProm.mCache.CacheMap
	require.Equal(t, 2, len(entriesMap))

	eInfo := entrySignature{
		Name:   "test_Bytes",
		Labels: entryLabels1,
	}

	eInfoBytes := generateCacheKey(&eInfo)
	encodeProm.mCache.Mu.Lock()
	_, found := encodeProm.mCache.CacheMap[string(eInfoBytes)]
	encodeProm.mCache.Mu.Unlock()
	require.Equal(t, true, found)

	// wait a couple seconds so that the entry will expire
	time.Sleep(2 * time.Second)
	encodeProm.mCache.CleanupExpiredEntries(encodeProm.expiryTime, encodeProm)
	entriesMap = encodeProm.mCache.CacheMap
	encodeProm.mCache.Mu.Lock()
	require.Equal(t, 0, len(entriesMap))
	encodeProm.mCache.Mu.Unlock()
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
