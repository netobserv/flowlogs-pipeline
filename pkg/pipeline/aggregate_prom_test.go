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
package pipeline

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/aggregate"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

// Test_Extract_Encode tests the integration between extract_aggregate and encode_prom.
// The test sends flows in 2 batches. Each batch is passed through the extractor and the encoder.
// The output of each stage is verified.
// The output of the 2nd batch depends on the 1st batch.
func Test_Extract_Encode(t *testing.T) {
	// Setup
	yamlConfig := `
pipeline:
 - name: extract
 - name: encode
parameters:
 - name: extract
   extract:
     type: aggregates
     aggregates:
       rules:
         - name: bandwidth_sum
           groupByKeys:
           - service
           operationType: sum
           operationKey: bytes
         - name: bandwidth_count
           groupByKeys:
           - service
           operationType: count
           operationKey:
         - name: bandwidth_raw_values
           groupByKeys:
           - service
           operationType: raw_values
           operationKey: bytes
 - name: encode
   encode:
     type: prom
     prom:
       prefix: test_
       expiryTime: 1s
       metrics:
         - name: flow_count
           type: counter
           filters: [{key: name, value: bandwidth_count}]
           valueKey: recent_count
           labels:
             - service
         - name: bytes_sum
           type: counter
           filters: [{key: name, value: bandwidth_sum}]
           valueKey: recent_op_value
           labels:
             - service
         - name: bytes_histogram
           type: agg_histogram
           filters: [{key: name, value: bandwidth_raw_values}]
           valueKey: recent_raw_values
           labels:
             - service
`
	var err error

	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	extractAggregate, err := extract.NewExtractAggregate(cfg.Parameters[0])
	require.NoError(t, err)

	promEncode, err := encode.NewEncodeProm(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[1])
	require.Equal(t, err, nil)

	// Test cases
	tests := []struct {
		name           string
		inputBatch     []config.GenericMap
		expectedAggs   []config.GenericMap
		expectedEncode []string
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
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 30, 2, nil, 30, 2),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 3, 2, nil, 3, 2),
				test.CreateMockAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 2, 2, nil, 2, 2),
				test.CreateMockAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 2, 2, nil, 2, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "http", aggregate.OperationRawValues, 0, 2, []float64{10, 20}, 0, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "tcp", aggregate.OperationRawValues, 0, 2, []float64{1, 2}, 0, 2),
			},
			expectedEncode: []string{
				`test_flow_count{service="http"} 2`,
				`test_flow_count{service="tcp"} 2`,
				`test_bytes_sum{service="http"} 30`,
				`test_bytes_sum{service="tcp"} 3`,
				`test_bytes_histogram_bucket{service="http",le="10"} 1`,
				`test_bytes_histogram_bucket{service="http",le="+Inf"} 2`,
				`test_bytes_histogram_sum{service="http"} 30`,
				`test_bytes_histogram_count{service="http"} 2`,
				`test_bytes_histogram_bucket{service="tcp",le="1"} 1`,
				`test_bytes_histogram_bucket{service="tcp",le="2.5"} 2`,
				`test_bytes_histogram_sum{service="tcp"} 3`,
				`test_bytes_histogram_count{service="tcp"} 2`,
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
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "http", aggregate.OperationSum, 60, 3, nil, 30, 1),
				test.CreateMockAgg("bandwidth_sum", "bytes", "service", "tcp", aggregate.OperationSum, 12, 4, nil, 9, 2),
				test.CreateMockAgg("bandwidth_count", "", "service", "http", aggregate.OperationCount, 3, 3, nil, 1, 1),
				test.CreateMockAgg("bandwidth_count", "", "service", "tcp", aggregate.OperationCount, 4, 4, nil, 2, 2),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "http", aggregate.OperationRawValues, 0, 3, []float64{30}, 0, 1),
				test.CreateMockAgg("bandwidth_raw_values", "bytes", "service", "tcp", aggregate.OperationRawValues, 0, 4, []float64{4, 5}, 0, 2),
			},
			expectedEncode: []string{
				`test_flow_count{service="http"} 3`,
				`test_flow_count{service="tcp"} 4`,
				`test_bytes_sum{service="http"} 60`,
				`test_bytes_sum{service="tcp"} 12`,
				`test_bytes_histogram_bucket{service="http",le="10"} 1`,
				`test_bytes_histogram_bucket{service="http",le="+Inf"} 3`,
				`test_bytes_histogram_sum{service="http"} 60`,
				`test_bytes_histogram_count{service="http"} 3`,
				`test_bytes_histogram_bucket{service="tcp",le="1"} 1`,
				`test_bytes_histogram_bucket{service="tcp",le="2.5"} 2`,
				`test_bytes_histogram_bucket{service="tcp",le="5"} 4`,
				`test_bytes_histogram_sum{service="tcp"} 12`,
				`test_bytes_histogram_count{service="tcp"} 4`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualAggs := extractAggregate.Extract(tt.inputBatch)
			// Since the order of the elements in the returned slice from Extract() and Encode() is non-deterministic,
			// we use ElementsMatch() rather than Equals()
			require.ElementsMatch(t, tt.expectedAggs, actualAggs)

			for _, aa := range actualAggs {
				promEncode.Encode(aa)
			}
			exposed := test.ReadExposedMetrics(t, promEncode.(*encode.EncodeProm).Gatherer())

			for _, expected := range tt.expectedEncode {
				require.Contains(t, exposed, expected)
			}
		})
	}
}
