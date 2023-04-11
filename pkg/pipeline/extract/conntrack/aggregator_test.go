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

	// invalid first agg
	_, err = newAggregator(api.OutputField{
		Operation: "first",
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
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", false, float64(0)}},
		},
		{
			name:        "Default input",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum", SplitAB: true},
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", true, float64(0)}},
		},
		{
			name:        "Custom input",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum", Input: "MyInput"},
			expected:    &aSum{aggregateBase{"MyInput", "MyAgg", false, float64(0)}},
		},
		{
			name:        "OperationType sum",
			outputField: api.OutputField{Name: "MyAgg", Operation: "sum"},
			expected:    &aSum{aggregateBase{"MyAgg", "MyAgg", false, float64(0)}},
		},
		{
			name:        "OperationType count",
			outputField: api.OutputField{Name: "MyAgg", Operation: "count"},
			expected:    &aCount{aggregateBase{"MyAgg", "MyAgg", false, float64(0)}},
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
		{
			name:        "Default first",
			outputField: api.OutputField{Name: "MyCp", Operation: "first"},
			expected:    &aFirst{aggregateBase{"MyCp", "MyCp", false, nil}},
		},
		{
			name:        "Custom input first",
			outputField: api.OutputField{Name: "MyCp", Operation: "first", Input: "MyInput"},
			expected:    &aFirst{aggregateBase{"MyInput", "MyCp", false, nil}},
		},
		{
			name:        "Default last",
			outputField: api.OutputField{Name: "MyCp", Operation: "last"},
			expected:    &aLast{aggregateBase{"MyCp", "MyCp", false, nil}},
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
		{Name: "FirstFlowDirection", Operation: "first", Input: "FlowDirection"},
		{Name: "LastFlowDirection", Operation: "last", Input: "FlowDirection"},
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
	flowDirA := 0
	flowDirB := 1

	table := []struct {
		name      string
		flowLog   config.GenericMap
		direction direction
		expected  map[string]interface{}
	}{
		{
			name:      "flowLog 1",
			flowLog:   newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDirA, 100, 10, false),
			direction: dirAB,
			expected:  map[string]interface{}{"Bytes_AB": float64(100), "Bytes_BA": float64(0), "Packets": float64(10), "maxFlowLogBytes": float64(100), "minFlowLogBytes": float64(100), "numFlowLogs": float64(1), "FirstFlowDirection": 0, "LastFlowDirection": 0},
		},
		{
			name:      "flowLog 2",
			flowLog:   newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDirB, 200, 20, false),
			direction: dirBA,
			expected:  map[string]interface{}{"Bytes_AB": float64(100), "Bytes_BA": float64(200), "Packets": float64(30), "maxFlowLogBytes": float64(200), "minFlowLogBytes": float64(100), "numFlowLogs": float64(2), "FirstFlowDirection": 0, "LastFlowDirection": 1},
		},
	}

	conn := NewConnBuilder(nil).Build()
	for _, agg := range aggs {
		agg.addField(conn)
	}
	expectedInits := map[string]interface{}{"Bytes_AB": float64(0), "Bytes_BA": float64(0), "Packets": float64(0), "maxFlowLogBytes": float64(-math.MaxFloat64), "minFlowLogBytes": float64(math.MaxFloat64), "numFlowLogs": float64(0), "FirstFlowDirection": nil, "LastFlowDirection": nil}
	require.Equal(t, expectedInits, conn.(*connType).aggFields)

	for i, test := range table {
		t.Run(test.name, func(t *testing.T) {
			for _, agg := range aggs {
				agg.update(conn, test.flowLog, test.direction, i == 0)
			}
			require.Equal(t, test.expected, conn.(*connType).aggFields)
		})
	}
}
