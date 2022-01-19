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
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.ibm.com/MCNM/observability/flowlogs2metrics/pkg/config"
	"github.ibm.com/MCNM/observability/flowlogs2metrics/pkg/test"
	"testing"
)

const testConfig = `---
log-level: debug
pipeline:
  encode:
    type: prom
    prom:
      port: 9103
      prefix: test_
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

func initNewEncodeProm(t *testing.T) Encoder {
	v := test.InitConfig(t, testConfig)
	val := v.Get("pipeline.encode.prom")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)

	config.Opt.PipeLine.Encode.Prom = string(b)
	newEncode, err := NewEncodeProm()
	require.Equal(t, err, nil)
	return newEncode
}

func Test_NewEncodeProm(t *testing.T) {
	newEncode := initNewEncodeProm(t)
	encodeProm := newEncode.(*encodeProm)
	require.Equal(t, encodeProm.port, ":9103")
	require.Equal(t, encodeProm.prefix, "test_")
	require.Equal(t, len(encodeProm.gauges), 1)
	require.Equal(t, len(encodeProm.counters), 1)
	require.Equal(t, len(encodeProm.histograms), 1)

	gauges := encodeProm.gauges
	assert.Contains(t, gauges, "Bytes")
	gInfo := gauges["Bytes"]
	require.Equal(t, gInfo.input, "bytes")
	expectedList := []string{"srcAddr", "dstAddr", "srcPort"}
	require.Equal(t, gInfo.tags, expectedList)

	counters := encodeProm.counters
	assert.Contains(t, counters, "Packets")
	cInfo := counters["Packets"]
	require.Equal(t, cInfo.input, "packets")
	expectedList = []string{"srcAddr", "dstAddr", "dstPort"}
	require.Equal(t, cInfo.tags, expectedList)

	entry := test.GetExtractMockEntry()
	input := []config.GenericMap{entry}
	output := encodeProm.Encode(input)

	entryLabels1 := make(map[string]string, 3)
	entryLabels2 := make(map[string]string, 3)
	entryLabels1["srcAddr"] = "10.1.2.3"
	entryLabels1["dstAddr"] = "10.1.2.4"
	entryLabels1["srcPort"] = "9001"
	entryLabels2["srcAddr"] = "10.1.2.3"
	entryLabels2["dstAddr"] = "10.1.2.4"
	entryLabels2["dstPort"] = "39504"
	gEntryInfo1 := gaugeEntryInfo{
		gaugeName:  "test_Bytes",
		gaugeValue: float64(1234),
		labels:     entryLabels1,
	}
	gEntryInfo2 := counterEntryInfo{
		counterName:  "test_Packets",
		counterValue: float64(34),
		labels:     entryLabels2,
	}
	expectedOutput := []interface{}{gEntryInfo1, gEntryInfo2}
	require.Equal(t, output, expectedOutput)
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

	newEncode := &encodeProm{
		port:   ":0000",
		prefix: "test_",
		gauges: map[string]gaugeInfo{
			"gauge": {
				input: "test_aggregate_value",
				tags:  []string{"by", "aggregate"},
			},
		},
	}

	output := newEncode.Encode(metrics)

	gEntryInfo1 := gaugeEntryInfo{
		gaugeName:  "test_gauge",
		gaugeValue: float64(7),
		labels: map[string]string{
			"by":        "[dstIP srcIP]",
			"aggregate": "20.0.0.2,10.0.0.1",
		},
	}

	expectedOutput := []interface{}{gEntryInfo1}
	require.Equal(t, output, expectedOutput)
}
