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

package conntrack

import (
	"math"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestNewAggregator_Invalid(t *testing.T) {
	var err error

	// Empty Name
	_, err = newAggregator(api.OutputField{
		Operation: "sum",
		SplitAB:   true,
		Input:     "Input",
	})
	require.NotNil(t, err)

	// unknown OperationType
	_, err = newAggregator(api.OutputField{
		Name:      "MyAgg",
		Operation: "unknown",
		SplitAB:   true,
		Input:     "Input",
	})
	require.NotNil(t, err)
}

func TestNewAggregator_Valid(t *testing.T) {
	table := []struct {
		name        string
		outputField api.OutputField
		expected    aggregator
	}{
		{
			name:        "Default SplitAB",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum"},
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", false, 0}},
		},
		{
			name:        "Default input",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum", SplitAB: true},
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", true, 0}},
		},
		{
			name:        "Custom input",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum", Input: "MyInput"},
			expected:    &aSum{aggregateBase{"MyInput", "MyAgg", false, 0}},
		},
		{
			name:        "OperationType sum",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum"},
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", false, 0}},
		},
		{
			name:        "OperationType count",
			outputField: api.OutputField{Name: "MyAgg", Operation: "count"},
			expected:    &aCount{aggregateBase{"MyAgg", "MyAgg", false, 0}},
		},
		{
			name:        "OperationType max",
			outputField: api.OutputField{Name: "MyAgg", Operation: "max"},
			expected:    &aMax{aggregateBase{"MyAgg", "MyAgg", false, -math.MaxFloat64}},
		},
		{
			name:        "OperationType min",
			outputField: api.OutputField{Name: "MyAgg", Operation: "min"},
			expected:    &aMin{aggregateBase{"MyAgg", "MyAgg", false, math.MaxFloat64}},
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			agg, err := newAggregator(test.outputField)
			require.NoError(t, err)
			require.Equal(t, test.expected, agg)
		})
	}
}

func TestAddField_and_Update(t *testing.T) {
	ofs := []api.OutputField{
		{Name: "Bytes", Operation: "sum", SplitAB: true},
		{Name: "Packets", Operation: "sum", SplitAB: false},
		{Name: "numFlowLogs", Operation: "count"},
		{Name: "minFlowLogBytes", Operation: "min", Input: "Bytes"},
		{Name: "maxFlowLogBytes", Operation: "max", Input: "Bytes"},
	}
	var aggs []aggregator
	for _, of := range ofs {
		agg, err := newAggregator(of)
		require.NoError(t, err)
		aggs = append(aggs, agg)
	}

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 1
	portB := 9002
	protocolA := 6

	table := []struct {
		name      string
		flowLog   config.GenericMap
		direction direction
		expected  map[string]float64
	}{
		{
			name:      "flowLog 1",
			flowLog:   newMockFlowLog(ipA, portA, ipB, portB, protocolA, 100, 10, false),
			direction: dirAB,
			expected:  map[string]float64{"Bytes_AB": 100, "Bytes_BA": 0, "Packets": 10, "maxFlowLogBytes": 100, "minFlowLogBytes": 100, "numFlowLogs": 1},
		},
		{
			name:      "flowLog 2",
			flowLog:   newMockFlowLog(ipA, portA, ipB, portB, protocolA, 200, 20, false),
			direction: dirBA,
			expected:  map[string]float64{"Bytes_AB": 100, "Bytes_BA": 200, "Packets": 30, "maxFlowLogBytes": 200, "minFlowLogBytes": 100, "numFlowLogs": 2},
		},
	}

	conn := NewConnBuilder(nil).Build()
	for _, agg := range aggs {
		agg.addField(conn)
	}
	expectedInits := map[string]float64{"Bytes_AB": 0, "Bytes_BA": 0, "Packets": 0, "maxFlowLogBytes": -math.MaxFloat64, "minFlowLogBytes": math.MaxFloat64, "numFlowLogs": 0}
	require.Equal(t, expectedInits, conn.(*connType).aggFields)

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			for _, agg := range aggs {
				agg.update(conn, test.flowLog, test.direction)
			}
			require.Equal(t, test.expected, conn.(*connType).aggFields)
		})
	}
}
