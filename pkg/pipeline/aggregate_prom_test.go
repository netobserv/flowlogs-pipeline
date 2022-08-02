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
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/aggregate"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func createEncodeOutput(name string, labels map[string]string, value interface{}) config.GenericMap {
	gm := config.GenericMap{
		"Name":   name,
		"Labels": labels,
		"value":  value,
	}
	return gm
}

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
       - name: bandwidth_sum
         by:
         - service
         operation: sum
         recordkey: bytes

       - name: bandwidth_count
         by:
         - service
         operation: count
         recordkey: 

       - name: bandwidth_raw_values
         by:
         - service
         operation: raw_values
         recordkey: bytes
 - name: encode
   encode:
     type: prom
     prom:
       port: 9103
       prefix: test_
       expiryTime: 1
       metrics:
         - name: flow_count
           type: counter
           filter: {key: name, value: bandwidth_count}
           valueKey: recent_count
           labels:
             - service

         - name: bytes_sum
           type: counter
           filter: {key: name, value: bandwidth_sum}
           valueKey: recent_op_value
           labels:
             - service

         - name: bytes_histogram
           type: histogram
           filter: {key: name, value: bandwidth_raw_values}
           valueKey: recent_raw_values
           labels:
             - service
`
	var err error

	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	extractAggregate, err := extract.NewExtractAggregate(cfg.Parameters[0])
	require.NoError(t, err)

	promEncode, err := encode.NewEncodeProm(cfg.Parameters[1])
	require.Equal(t, err, nil)

	// Test cases
	tests := []struct {
		name           string
		inputBatch     []config.GenericMap
		expectedAggs   []config.GenericMap
		expectedEncode []config.GenericMap
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
			expectedEncode: []config.GenericMap{
				createEncodeOutput("test_flow_count", map[string]string{"service": "http"}, 2),
				createEncodeOutput("test_flow_count", map[string]string{"service": "tcp"}, 2),
				createEncodeOutput("test_bytes_sum", map[string]string{"service": "http"}, 30.0),
				createEncodeOutput("test_bytes_sum", map[string]string{"service": "tcp"}, 3.0),
				createEncodeOutput("test_bytes_histogram", map[string]string{"service": "http"}, []float64{10, 20}),
				createEncodeOutput("test_bytes_histogram", map[string]string{"service": "tcp"}, []float64{1, 2}),
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
			expectedEncode: []config.GenericMap{
				createEncodeOutput("test_flow_count", map[string]string{"service": "http"}, 1),
				createEncodeOutput("test_flow_count", map[string]string{"service": "tcp"}, 2),
				createEncodeOutput("test_bytes_sum", map[string]string{"service": "http"}, 30.0),
				createEncodeOutput("test_bytes_sum", map[string]string{"service": "tcp"}, 9.0),
				createEncodeOutput("test_bytes_histogram", map[string]string{"service": "http"}, []float64{30}),
				createEncodeOutput("test_bytes_histogram", map[string]string{"service": "tcp"}, []float64{4, 5}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualAggs := extractAggregate.Extract(tt.inputBatch)
			// Since the order of the elements in the returned slice from Extract() and Encode() is non-deterministic,
			// we use ElementsMatch() rather than Equals()
			require.ElementsMatch(t, tt.expectedAggs, actualAggs)

			promEncode.Encode(actualAggs)
			require.ElementsMatch(t, tt.expectedEncode, promEncode.(*encode.EncodeProm).PrevRecords)
		})
	}
}
