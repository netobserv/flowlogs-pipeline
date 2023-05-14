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

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/aggregate"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

// This tests extract_aggregate as a whole. It can be thought of as an integration test between extract_aggregate.go and
// aggregate.go and aggregates.go. The test sends flows in 2 batches and verifies the extractor's output after each
// batch. The output of the 2nd batch depends on the 1st batch.
func Test_Extract(t *testing.T) {
	// Setup
	yamlConfig := `
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: aggregates
      aggregates:
        rules:
          - name: bandwidth_count
            groupByKeys:
            - service
            operationType: count
            operationKey: ""
          - name: bandwidth_sum
            groupByKeys:
            - service
            operationType: sum
            operationKey: bytes
          - name: bandwidth_max
            groupByKeys:
            - service
            operationType: max
            operationKey: bytes
          - name: bandwidth_min
            groupByKeys:
            - service
            operationType: min
            operationKey: bytes
          - name: bandwidth_avg
            groupByKeys:
            - service
            operationType: avg
            operationKey: bytes
          - name: bandwidth_raw_values
            groupByKeys:
            - service
            operationType: raw_values
            operationKey: bytes
`
	var err error

	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	extractAggregate, err := NewExtractAggregate(cfg.Parameters[0])
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
				{"service": "http", "bytes": 10},
				{"service": "http", "bytes": 20},
				{"service": "tcp", "bytes": 1},
				{"service": "tcp", "bytes": 2},
			},
			expectedAggs: []config.GenericMap{
				test.CreateMockAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 2, 2, nil, 2, 2),
				test.CreateMockAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 2, 2, nil, 2, 2),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 30, 2, nil, 30, 2),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 3, 2, nil, 3, 2),
				test.CreateMockAgg("bandwidth_max", "bytes", "service", "http", aggregate.OperationMax, 20, 2, nil, 20, 2),
				test.CreateMockAgg("bandwidth_max", "bytes", "service", "tcp", aggregate.OperationMax, 2, 2, nil, 2, 2),
				test.CreateMockAgg("bandwidth_min", "bytes", "service", "http", aggregate.OperationMin, 10, 2, nil, 10, 2),
				test.CreateMockAgg("bandwidth_min", "bytes", "service", "tcp", aggregate.OperationMin, 1, 2, nil, 1, 2),
				test.CreateMockAgg("bandwidth_avg", "bytes", "service", "http", aggregate.OperationAvg, 15, 2, nil, 15, 2),
				test.CreateMockAgg("bandwidth_avg", "bytes", "service", "tcp", aggregate.OperationAvg, 1.5, 2, nil, 1.5, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "http", aggregate.OperationRawValues, 0, 2, []float64{10, 20}, 0, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "tcp", aggregate.OperationRawValues, 0, 2, []float64{1, 2}, 0, 2),
			},
		},
		{
			name: "batch2",
			inputBatch: []config.GenericMap{
				{"service": "http", "bytes": 30},
				{"service": "tcp", "bytes": 4},
				{"service": "tcp", "bytes": 5},
			},
			expectedAggs: []config.GenericMap{
				test.CreateMockAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 3, 3, nil, 1, 1),
				test.CreateMockAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 4, 4, nil, 2, 2),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 60, 3, nil, 30, 1),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 12, 4, nil, 9, 2),
				test.CreateMockAgg("bandwidth_max", "bytes", "service", "http", aggregate.OperationMax, 30, 3, nil, 30, 1),
				test.CreateMockAgg("bandwidth_max", "bytes", "service", "tcp", aggregate.OperationMax, 5, 4, nil, 5, 2),
				test.CreateMockAgg("bandwidth_min", "bytes", "service", "http", aggregate.OperationMin, 10, 3, nil, 30, 1),
				test.CreateMockAgg("bandwidth_min", "bytes", "service", "tcp", aggregate.OperationMin, 1, 4, nil, 4, 2),
				test.CreateMockAgg("bandwidth_avg", "bytes", "service", "http", aggregate.OperationAvg, 20, 3, nil, 30, 1),
				test.CreateMockAgg("bandwidth_avg", "bytes", "service", "tcp", aggregate.OperationAvg, 3, 4, nil, 4.5, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "http", aggregate.OperationRawValues, 0, 3, []float64{30}, 0, 1),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "tcp", aggregate.OperationRawValues, 0, 4, []float64{4, 5}, 0, 2),
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
