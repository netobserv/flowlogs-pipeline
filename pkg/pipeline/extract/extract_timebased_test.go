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

package extract

import (
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	tb "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/timebased"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func getMockTimebased1() timebased {
	tb := timebased{
		Filters: []tb.FilterStruct{
			{Rule: api.TimebasedFilterRule{
				Name:          "TopK_Bytes1",
				IndexKey:      "SrcAddr",
				IndexKeys:     []string{"SrcAddr"},
				OperationType: "last",
				OperationKey:  "Bytes",
				TopK:          3,
				TimeInterval:  api.Duration{Duration: 10 * time.Second},
			}},
			{Rule: api.TimebasedFilterRule{
				Name:          "BotK_Bytes1",
				IndexKey:      "SrcAddr",
				IndexKeys:     []string{"SrcAddr"},
				OperationType: "avg",
				OperationKey:  "Bytes",
				TopK:          2,
				Reversed:      true,
				TimeInterval:  api.Duration{Duration: 15 * time.Second},
			}},
		},
		IndexKeyStructs: map[string]*tb.IndexKeyTable{},
	}
	return tb
}

var yamlConfigTopAvg = `
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes1
            operationType: last
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 3
            timeInterval: 10s
          - name: BotK_Bytes1
            operationType: avg
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 2
            reversed: true
            timeInterval: 15s
`

var yamlConfigSum = `
pipeline:
  - name: extract2
parameters:
  - name: extract2
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes2
            operationType: sum
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 1
            timeInterval: 10s
`

var yamlConfigDiff = `
pipeline:
  - name: extract3
parameters:
  - name: extract3
    extract:
      type: timebased
      timebased:
        rules:
          - name: BotK_Bytes3
            operationType: diff
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 1
            reversed: true
            timeInterval: 10s
`

var yamlConfigMax = `
pipeline:
  - name: extract4
parameters:
  - name: extract4
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes4
            operationType: max
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 1
            timeInterval: 10s
`

var yamlConfigMinReversed = `
pipeline:
  - name: extract5
parameters:
  - name: extract5
    extract:
      type: timebased
      timebased:
        rules:
          - name: BotK_Bytes5
            operationType: min
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 1
            reversed: true
            timeInterval: 10s
`

var yamlConfigAllFlows = `
pipeline:
  - name: extract6
parameters:
  - name: extract6
    extract:
      type: timebased
      timebased:
        rules:
          - name: All_Bytes6
            operationType: sum
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            timeInterval: 10s
`

var yamlConfigCount = `
pipeline:
  - name: extract7
parameters:
  - name: extract7
    extract:
      type: timebased
      timebased:
        rules:
          - name: Count_Flows
            operationType: count
            operationKey: Bytes
            indexKeys: ['SrcAddr']
            topK: 5
            timeInterval: 10s
`

var yamlConfigMultipleKeys = `
pipeline:
  - name: extract8
parameters:
  - name: extract8
    extract:
      type: timebased
      timebased:
        rules:
          - name: BotK_SrcDst_Bytes
            operationType: avg
            operationKey: Bytes
            indexKeys: ['SrcAddr','DstAddr','Direction']
            topK: 2
            timeInterval: 10s
`

func initTimebased(t *testing.T, yamlConfig string) *timebased {
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)
	extractor, err := NewExtractTimebased(cfg.Parameters[0])
	require.NoError(t, err)

	return extractor.(*timebased)
}

func Test_NewExtractTimebased(t *testing.T) {

	tb := initTimebased(t, yamlConfigTopAvg)
	require.NotNil(t, tb)
	expectedTimebased := getMockTimebased1()
	require.Equal(t, expectedTimebased.Filters[0].Rule, tb.Filters[0].Rule)
	require.Equal(t, expectedTimebased.Filters[1].Rule, tb.Filters[1].Rule)
}

func Test_ExtractTimebasedTopAvg(t *testing.T) {
	tb := initTimebased(t, yamlConfigTopAvg)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 5, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "TopK_Bytes1",
			"operation": api.FilterOperationLast,
			"Bytes":     float64(1000),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.4",
		},
		{
			"name":      "TopK_Bytes1",
			"operation": api.FilterOperationLast,
			"Bytes":     float64(900),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.3",
		},
		{
			"name":      "TopK_Bytes1",
			"operation": api.FilterOperationLast,
			"Bytes":     float64(800),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.2",
		},
		{
			"name":      "BotK_Bytes1",
			"operation": api.FilterOperationAvg,
			"Bytes":     float64(400),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.1",
		},
		{
			"name":      "BotK_Bytes1",
			"operation": api.FilterOperationAvg,
			"Bytes":     float64(500),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.2",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedSum(t *testing.T) {
	tb := initTimebased(t, yamlConfigSum)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "TopK_Bytes2",
			"operation": api.FilterOperationSum,
			"Bytes":     float64(1800),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.3",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedDiff(t *testing.T) {
	tb := initTimebased(t, yamlConfigDiff)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "BotK_Bytes3",
			"operation": api.FilterOperationDiff,
			"Bytes":     float64(0),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.4",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedMax(t *testing.T) {
	tb := initTimebased(t, yamlConfigMax)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "TopK_Bytes4",
			"operation": api.FilterOperationMax,
			"Bytes":     float64(1000),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.4",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedMinReversed(t *testing.T) {
	tb := initTimebased(t, yamlConfigMinReversed)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "BotK_Bytes5",
			"operation": api.FilterOperationMin,
			"Bytes":     float64(100),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.1",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedAllFlows(t *testing.T) {
	tb := initTimebased(t, yamlConfigAllFlows)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 4, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "All_Bytes6",
			"operation": api.FilterOperationSum,
			"Bytes":     float64(1200),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.1",
		},
		{
			"name":      "All_Bytes6",
			"operation": api.FilterOperationSum,
			"Bytes":     float64(1500),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.2",
		},
		{
			"name":      "All_Bytes6",
			"operation": api.FilterOperationSum,
			"Bytes":     float64(1800),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.3",
		},
		{
			"name":      "All_Bytes6",
			"operation": api.FilterOperationSum,
			"Bytes":     float64(1000),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.4",
		},
	}
	for _, configMap := range expectedOutput {
		require.Contains(t, output, configMap)
	}
}

func Test_ExtractTimebasedCount(t *testing.T) {
	tb := initTimebased(t, yamlConfigCount)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 4, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "Count_Flows",
			"operation": api.FilterOperationCnt,
			"Bytes":     float64(3),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.1",
		},
		{
			"name":      "Count_Flows",
			"operation": api.FilterOperationCnt,
			"Bytes":     float64(3),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.2",
		},
		{
			"name":      "Count_Flows",
			"operation": api.FilterOperationCnt,
			"Bytes":     float64(3),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.3",
		},
		{
			"name":      "Count_Flows",
			"operation": api.FilterOperationCnt,
			"Bytes":     float64(1),
			"index_key": "SrcAddr",
			"SrcAddr":   "10.0.0.4",
		},
	}
	for _, configMap := range expectedOutput {
		require.Contains(t, output, configMap)
	}
}

func Test_ExtractTimebasedMultiple(t *testing.T) {
	tb := initTimebased(t, yamlConfigMultipleKeys)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries3()
	output := tb.Extract(entries)
	require.Equal(t, 2, len(output))
	expectedOutput := []config.GenericMap{
		{
			"name":      "BotK_SrcDst_Bytes",
			"operation": api.FilterOperationAvg,
			"Bytes":     float64(1000),
			"index_key": "SrcAddr,DstAddr,Direction",
			"SrcAddr":   "10.0.0.4",
			"DstAddr":   "11.0.0.2",
			"Direction": "1",
		},
		{
			"name":      "BotK_SrcDst_Bytes",
			"operation": api.FilterOperationAvg,
			"Bytes":     float64(500),
			"index_key": "SrcAddr,DstAddr,Direction",
			"SrcAddr":   "10.0.0.2",
			"DstAddr":   "11.0.0.1",
			"Direction": "0",
		},
	}
	require.Equal(t, expectedOutput, output)
}
