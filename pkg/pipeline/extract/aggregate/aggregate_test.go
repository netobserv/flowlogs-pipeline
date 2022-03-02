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

package aggregate

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func GetMockAggregate() Aggregate {
	aggregate := Aggregate{
		Definition: api.AggregateDefinition{
			Name:      "Avg by src and dst IP's",
			By:        api.AggregateBy{"dstIP", "srcIP"},
			Operation: "avg",
			RecordKey: "value",
		},
		Groups: map[NormalizedValues]*GroupState{},
	}
	return aggregate
}

func getMockLabels(reverseOrder bool) Labels {
	var labels Labels
	if reverseOrder {
		labels = Labels{
			"srcIP": "10.0.0.1",
			"dstIP": "20.0.0.2",
		}
	} else {
		labels = Labels{
			"dstIP": "20.0.0.2",
			"srcIP": "10.0.0.1",
		}
	}

	return labels
}

func Test_getNormalizedValues(t *testing.T) {
	expectedLabels := NormalizedValues("20.0.0.2,10.0.0.1")

	labels := getMockLabels(false)

	normalizedValues := labels.getNormalizedValues()

	require.Equal(t, expectedLabels, normalizedValues)
	labels = getMockLabels(true)

	normalizedValues = labels.getNormalizedValues()

	require.Equal(t, expectedLabels, normalizedValues)
}

func Test_LabelsFromEntry(t *testing.T) {
	aggregate := Aggregate{
		Definition: api.AggregateDefinition{
			By:        api.AggregateBy{"dstIP", "srcIP"},
			Operation: "count",
			RecordKey: "",
		},
	}
	expectedLabels := getMockLabels(false)
	entry := test.GetIngestMockEntry(false)

	labels, allLabelsFound := aggregate.LabelsFromEntry(entry)

	require.Equal(t, allLabelsFound, true)
	require.Equal(t, labels, expectedLabels)
}

func Test_FilterEntry(t *testing.T) {
	aggregate := GetMockAggregate()
	entry := test.GetIngestMockEntry(false)

	err, _ := aggregate.FilterEntry(entry)

	require.Equal(t, err, nil)
	entry = test.GetIngestMockEntry(true)

	err, _ = aggregate.FilterEntry(entry)

	require.EqualError(t, err, "missing keys in entry")
}

func Test_Evaluate(t *testing.T) {
	aggregate := GetMockAggregate()
	entry1 := test.GetIngestMockEntry(false)
	entry2 := test.GetIngestMockEntry(false)
	entry3 := test.GetIngestMockEntry(true)
	entries := []config.GenericMap{entry1, entry2, entry3}
	labels, _ := aggregate.LabelsFromEntry(entry1)
	normalizedValues := labels.getNormalizedValues()

	err := aggregate.Evaluate(entries)

	require.Equal(t, err, nil)
	require.Equal(t, aggregate.Groups[normalizedValues].count, 2)
	require.Equal(t, aggregate.Groups[normalizedValues].value, float64(7))
}

func Test_GetMetrics(t *testing.T) {
	aggregate := GetMockAggregate()
	entry1 := test.GetIngestMockEntry(false)
	entry2 := test.GetIngestMockEntry(false)
	entry3 := test.GetIngestMockEntry(true)
	entries := []config.GenericMap{entry1, entry2, entry3}

	_ = aggregate.Evaluate(entries)
	metrics := aggregate.GetMetrics()

	require.Equal(t, len(metrics), 1)
	require.Equal(t, metrics[0]["name"], aggregate.Definition.Name)
	valueFloat64, _ := strconv.ParseFloat(fmt.Sprintf("%s", metrics[0]["value"]), 64)
	require.Equal(t, valueFloat64, float64(7))
}
