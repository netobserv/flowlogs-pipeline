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
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/aggregate"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"testing"
)

func createAgg(name, recordKey, by, agg, op string, value float64, count int, rrv []float64) config.GenericMap {
	valueString := fmt.Sprintf("%f", value)
	return config.GenericMap{
		"name":                        name,
		"record_key":                  recordKey,
		"by":                          by,
		"aggregate":                   agg,
		by:                            agg,
		"operation":                   aggregate.Operation(op),
		"value":                       valueString,
		fmt.Sprintf("%v_value", name): valueString,
		"recentRawValues":             rrv,
		"count":                       fmt.Sprintf("%v", count),
	}
}

// This tests extract_aggregate as a whole. It can be thought of as an integration test between extract_aggregate.go and
// aggregate.go and aggregates.go. The test sends flows in 2 batches and verifies the extractor's output after each
// batch. The output of the 2nd batch depends on the 1st batch.
func Test_Extract(t *testing.T) {
	// Setup
	yamlConfig := `
aggregates:
- name: bandwidth_count
  by:
  - service
  operation: count
  recordkey: ""

- name: bandwidth_sum
  by:
  - service
  operation: sum
  recordkey: bytes

- name: bandwidth_max
  by:
  - service
  operation: max
  recordkey: bytes
`
	var err error
	yamlData := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(yamlConfig), &yamlData)
	require.NoError(t, err)
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	jsonBytes, err := json.Marshal(yamlData["aggregates"])
	require.NoError(t, err)
	config.Opt.PipeLine.Extract.Aggregates = string(jsonBytes)

	extractAggregate, err := NewExtractAggregate()
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		name         string
		inputBatch   []config.GenericMap
		expectedAggs []config.GenericMap
	}{
		{
			name: "batch1",
			inputBatch: []config.GenericMap{
				{"service": "http", "bytes": 10.0},
				{"service": "http", "bytes": 20.0},
				{"service": "tcp", "bytes": 1.0},
				{"service": "tcp", "bytes": 2.0},
			},
			expectedAggs: []config.GenericMap{
				createAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 2, 2, []float64{1.0, 1.0}),
				createAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 2, 2, []float64{1.0, 1.0}),
				createAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 30, 2, []float64{10.0, 20.0}),
				createAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 3, 2, []float64{1.0, 2.0}),
				createAgg("bandwidth_max", "bytes", "service", "http", aggregate.OperationMax, 20, 2, []float64{10.0, 20.0}),
				createAgg("bandwidth_max", "bytes", "service", "tcp", aggregate.OperationMax, 2, 2, []float64{1.0, 2.0}),
			},
		},
		{
			name: "batch2",
			inputBatch: []config.GenericMap{
				{"service": "http", "bytes": 30.0},
				{"service": "tcp", "bytes": 4.0},
				{"service": "tcp", "bytes": 5.0},
			},
			expectedAggs: []config.GenericMap{
				createAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 3, 3, []float64{1.0}),
				createAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 4, 4, []float64{1.0, 1.0}),
				createAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 60, 3, []float64{30.0}),
				createAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 12, 4, []float64{4.0, 5.0}),
				createAgg("bandwidth_max", "bytes", "service", "http", aggregate.OperationMax, 30, 3, []float64{30.0}),
				createAgg("bandwidth_max", "bytes", "service", "tcp", aggregate.OperationMax, 5, 4, []float64{4.0, 5.0}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualAggs := extractAggregate.Extract(tt.inputBatch)
			// Since the order of the elements in the returned slice from Extract() is non-deterministic, we use
			// ElementsMatch() rather than Equals()
			require.ElementsMatch(t, tt.expectedAggs, actualAggs)
		})
	}
}
