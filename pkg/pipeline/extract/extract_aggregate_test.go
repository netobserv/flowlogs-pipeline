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
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/aggregate"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"testing"
)

func Test_Extract(t *testing.T) {
	yamlConfig := `
aggregates:
- name: bandwidth
  by:
  - service
  operation: sum
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

	input1 := []config.GenericMap{
		{"service": "http", "bytes": 10.0},
		{"service": "http", "bytes": 20.0},
		{"service": "tcp", "bytes": 1.0},
		{"service": "tcp", "bytes": 2.0},
	}
	expectedAggs1 := []config.GenericMap{
		{
			"name":            "bandwidth",
			"record_key":      "bytes",
			"by":              "service",
			"aggregate":       "http",
			"service":         "http",
			"operation":       aggregate.Operation(aggregate.OperationSum),
			"value":           "30.000000",
			"bandwidth_value": "30.000000",
			"recentRawValues": []float64{10.0, 20.0},
			"count":           "2",
		},
		{
			"name":            "bandwidth",
			"record_key":      "bytes",
			"by":              "service",
			"aggregate":       "tcp",
			"service":         "tcp",
			"operation":       aggregate.Operation(aggregate.OperationSum),
			"value":           "3.000000",
			"bandwidth_value": "3.000000",
			"recentRawValues": []float64{1.0, 2.0},
			"count":           "2",
		},
	}
	actualAggs1 := extractAggregate.Extract(input1)
	require.Equal(t, expectedAggs1, actualAggs1)

	input2 := []config.GenericMap{
		{"service": "http", "bytes": 30.0},
		{"service": "tcp", "bytes": 4.0},
		{"service": "tcp", "bytes": 5.0},
	}
	expectedAggs2 := []config.GenericMap{
		{
			"name":            "bandwidth",
			"record_key":      "bytes",
			"by":              "service",
			"aggregate":       "http",
			"service":         "http",
			"operation":       aggregate.Operation(aggregate.OperationSum),
			"value":           "60.000000",
			"bandwidth_value": "60.000000",
			"recentRawValues": []float64{30.0},
			"count":           "3",
		},
		{
			"name":            "bandwidth",
			"record_key":      "bytes",
			"by":              "service",
			"aggregate":       "tcp",
			"service":         "tcp",
			"operation":       aggregate.Operation(aggregate.OperationSum),
			"value":           "12.000000",
			"bandwidth_value": "12.000000",
			"recentRawValues": []float64{4.0, 5.0},
			"count":           "4",
		},
	}
	actualAggs2 := extractAggregate.Extract(input2)
	require.Equal(t, expectedAggs2, actualAggs2)

}
