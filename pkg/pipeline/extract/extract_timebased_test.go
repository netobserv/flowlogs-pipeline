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

package extract

import (
	"fmt"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/timebased"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func GetMockTimebased() ExtractTimebased {
	tb := ExtractTimebased{
		Filters: []timebased.FilterStruct{
			{Rule: api.TimebasedFilterRule{
				Name:         "TopK_Bytes",
				RecordKey:    "SrcAddr",
				Operation:    "last",
				OperationKey: "Bytes",
				TopK:         3,
				TimeInterval: 10,
			}},
			{Rule: api.TimebasedFilterRule{
				Name:         "BotK_Bytes",
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

func initTimebased(t *testing.T) *ExtractTimebased {
	var yamlConfig = `
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: timebased
      timebased:
        rules:
          - Name: TopK_Bytes
            Operation: last
            OperationKey: Bytes
            RecordKey: SrcAddr
            TopK: 3
            TimeInterval: 10
          - Name: BotK_Bytes
            Operation: avg
            OperationKey: Bytes
            RecordKey: SrcAddr
            BotK: 2
            TimeInterval: 15
`
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)
	extractor, err := NewExtractTimebased(cfg.Parameters[0])
	require.NoError(t, err)

	return extractor.(*ExtractTimebased)
}

func Test_NewExtractTimebased(t *testing.T) {

	tb := initTimebased(t)
	require.NotNil(t, tb)
	expectedTimebased := GetMockTimebased()
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

func Test_ExtractTimebasedExtract(t *testing.T) {
	tb := initTimebased(t)
	require.NotNil(t, tb)
	entries := test.GetExtractGenericMaps()
	output := tb.Extract(entries)
	fmt.Printf("output = %v \n", output)
	require.Equal(t, 5, len(output))
	expectedOutput := []config.GenericMap{
		{
			"key_value":        "10.0.0.4",
			"name":             "TopK_Bytes",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(1000),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.3",
			"name":             "TopK_Bytes",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(900),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.2",
			"name":             "TopK_Bytes",
			"operation":        api.FilterOperation("last"),
			"operation_key":    "Bytes",
			"operation_result": float64(800),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.1",
			"name":             "BotK_Bytes",
			"operation":        api.FilterOperation("avg"),
			"operation_key":    "Bytes",
			"operation_result": float64(400),
			"record_key":       "SrcAddr",
		},
		{
			"key_value":        "10.0.0.2",
			"name":             "BotK_Bytes",
			"operation":        api.FilterOperation("avg"),
			"operation_key":    "Bytes",
			"operation_result": float64(500),
			"record_key":       "SrcAddr",
		},
	}
	require.Equal(t, expectedOutput, output)
}
