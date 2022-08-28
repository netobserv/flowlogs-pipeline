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

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/timebased"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func GetMockTimebased1() ExtractTimebased {
	tb := ExtractTimebased{
		Filters: []timebased.FilterStruct{
			{Rule: api.TimebasedFilterRule{
				Name:         "TopK_Bytes1",
				RecordKey:    "SrcAddr",
				Operation:    "last",
				OperationKey: "Bytes",
				TopK:         3,
				TimeInterval: 10,
			}},
			{Rule: api.TimebasedFilterRule{
				Name:         "BotK_Bytes1",
				RecordKey:    "SrcAddr",
				Operation:    "avg",
				OperationKey: "Bytes",
				BotK:         2,
				TimeInterval: 15,
			}},
		},
		RecordKeyStructs: map[string]*timebased.RecordKeyTable{},
	}
	return tb
}

var yamlConfig1 = `
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes1
            operation: last
            operationKey: Bytes
            recordKey: SrcAddr
            topK: 3
            timeInterval: 10
          - name: BotK_Bytes1
            operation: avg
            operationKey: Bytes
            recordKey: SrcAddr
            botK: 2
            timeInterval: 15
`

var yamlConfig2 = `
pipeline:
  - name: extract2
parameters:
  - name: extract2
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes2
            operation: sum
            operationKey: Bytes
            recordKey: SrcAddr
            topK: 1
            timeInterval: 10
`

var yamlConfig3 = `
pipeline:
  - name: extract3
parameters:
  - name: extract3
    extract:
      type: timebased
      timebased:
        rules:
          - name: BotK_Bytes3
            operation: diff
            operationKey: Bytes
            recordKey: SrcAddr
            botK: 1
            timeInterval: 10
`

var yamlConfig4 = `
pipeline:
  - name: extract4
parameters:
  - name: extract4
    extract:
      type: timebased
      timebased:
        rules:
          - name: TopK_Bytes4
            operation: max
            operationKey: Bytes
            recordKey: SrcAddr
            topK: 1
            timeInterval: 10
`

var yamlConfig5 = `
pipeline:
  - name: extract5
parameters:
  - name: extract5
    extract:
      type: timebased
      timebased:
        rules:
          - name: BotK_Bytes5
            operation: min
            operationKey: Bytes
            recordKey: SrcAddr
            botK: 1
            timeInterval: 10
`

var yamlConfig6 = `
pipeline:
  - name: extract6
parameters:
  - name: extract6
    extract:
      type: timebased
      timebased:
        rules:
          - name: All_Bytes6
            operation: sum
            operationKey: Bytes
            recordKey: SrcAddr
            timeInterval: 10
`

func initTimebased(t *testing.T, yamlConfig string) *ExtractTimebased {
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)
	extractor, err := NewExtractTimebased(cfg.Parameters[0])
	require.NoError(t, err)

	return extractor.(*ExtractTimebased)
}

func Test_NewExtractTimebased(t *testing.T) {

	tb := initTimebased(t, yamlConfig1)
	require.NotNil(t, tb)
	expectedTimebased := GetMockTimebased1()
	require.Equal(t, expectedTimebased.Filters[0].Rule.Name, tb.Filters[0].Rule.Name)
	require.Equal(t, expectedTimebased.Filters[0].Rule.Operation, tb.Filters[0].Rule.Operation)
	require.Equal(t, expectedTimebased.Filters[0].Rule.OperationKey, tb.Filters[0].Rule.OperationKey)
	require.Equal(t, expectedTimebased.Filters[0].Rule.TopK, tb.Filters[0].Rule.TopK)
	require.Equal(t, expectedTimebased.Filters[0].Rule.RecordKey, tb.Filters[0].Rule.RecordKey)
	require.Equal(t, expectedTimebased.Filters[0].Rule.TimeInterval, tb.Filters[0].Rule.TimeInterval)

	require.Equal(t, expectedTimebased.Filters[1].Rule.Name, tb.Filters[1].Rule.Name)
	require.Equal(t, expectedTimebased.Filters[1].Rule.Operation, tb.Filters[1].Rule.Operation)
	require.Equal(t, expectedTimebased.Filters[1].Rule.OperationKey, tb.Filters[1].Rule.OperationKey)
	require.Equal(t, expectedTimebased.Filters[1].Rule.BotK, tb.Filters[1].Rule.BotK)
	require.Equal(t, expectedTimebased.Filters[1].Rule.RecordKey, tb.Filters[1].Rule.RecordKey)
	require.Equal(t, expectedTimebased.Filters[1].Rule.TimeInterval, tb.Filters[1].Rule.TimeInterval)
}

func Test_ExtractTimebasedExtract1(t *testing.T) {
	tb := initTimebased(t, yamlConfig1)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 5, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.4",
			"name":             "TopK_Bytes1",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(1000),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.3",
			"name":             "TopK_Bytes1",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(900),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.2",
			"name":             "TopK_Bytes1",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(800),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.1",
			"name":             "BotK_Bytes1",
			"operation":        api.FilterOperation("avg"),
			"operation_key":    "Bytes",
			"operation_result": float64(400),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.2",
			"name":             "BotK_Bytes1",
			"operation":        api.FilterOperation("avg"),
			"operation_key":    "Bytes",
			"operation_result": float64(500),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedExtract2(t *testing.T) {
	tb := initTimebased(t, yamlConfig2)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.3",
			"name":             "TopK_Bytes2",
			"operation":        api.FilterOperation("sum"),
			"operation_key":    "Bytes",
			"operation_result": float64(1800),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedExtract3(t *testing.T) {
	tb := initTimebased(t, yamlConfig3)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.4",
			"name":             "BotK_Bytes3",
			"operation":        api.FilterOperation("diff"),
			"operation_key":    "Bytes",
			"operation_result": float64(0),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedExtract4(t *testing.T) {
	tb := initTimebased(t, yamlConfig4)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.4",
			"name":             "TopK_Bytes4",
			"operation":        api.FilterOperation("max"),
			"operation_key":    "Bytes",
			"operation_result": float64(1000),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedExtract5(t *testing.T) {
	tb := initTimebased(t, yamlConfig5)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 1, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.1",
			"name":             "BotK_Bytes5",
			"operation":        api.FilterOperation("min"),
			"operation_key":    "Bytes",
			"operation_result": float64(100),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}

func Test_ExtractTimebasedExtract6(t *testing.T) {
	tb := initTimebased(t, yamlConfig6)
	require.NotNil(t, tb)
	entries := test.GetExtractMockEntries2()
	output := tb.Extract(entries)
	require.Equal(t, 4, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.1",
			"name":             "All_Bytes6",
			"operation":        api.FilterOperation("sum"),
			"operation_key":    "Bytes",
			"operation_result": float64(1200),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.2",
			"name":             "All_Bytes6",
			"operation":        api.FilterOperation("sum"),
			"operation_key":    "Bytes",
			"operation_result": float64(1500),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.3",
			"name":             "All_Bytes6",
			"operation":        api.FilterOperation("sum"),
			"operation_key":    "Bytes",
			"operation_result": float64(1800),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.4",
			"name":             "All_Bytes6",
			"operation":        api.FilterOperation("sum"),
			"operation_key":    "Bytes",
			"operation_result": float64(1000),
			"record_key":       "SrcAddr",
		},
	}
	for _, configMap := range expectedOutput {
		require.Contains(t, output, configMap)
	}
}
