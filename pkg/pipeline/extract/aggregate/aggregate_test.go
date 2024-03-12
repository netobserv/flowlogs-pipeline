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
	"sync"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func GetMockAggregate() Aggregate {
	aggregate := Aggregate{
		definition: &api.AggregateDefinition{
			Name:          "Avg by src and dst IP's",
			GroupByKeys:   api.AggregateBy{"dstIP", "srcIP"},
			OperationType: "avg",
			OperationKey:  "value",
		},
		cache:      utils.NewTimedCache(0, nil),
		mutex:      &sync.Mutex{},
		expiryTime: 30 * time.Second,
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
		definition: &api.AggregateDefinition{
			GroupByKeys:   api.AggregateBy{"dstIP", "srcIP"},
			OperationType: "count",
			OperationKey:  "",
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

	_, _, err := aggregate.filterEntry(entry)
	require.Equal(t, err, nil)

	normalizedLabels, labels, err := aggregate.filterEntry(entry)
	require.Equal(t, err, nil)
	require.Equal(t, Labels{"srcIP": "10.0.0.1", "dstIP": "20.0.0.2"}, labels)
	require.Equal(t, NormalizedValues("20.0.0.2,10.0.0.1"), normalizedLabels)

	entry = test.GetIngestMockEntry(true)

	_, _, err = aggregate.filterEntry(entry)

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

	require.Equal(t, nil, err)
	require.Equal(t, 1, aggregate.cache.GetCacheLen())
	cacheEntry, found := aggregate.cache.GetCacheEntry(string(normalizedValues))
	gState := cacheEntry.(*GroupState)
	require.Equal(t, true, found)
	require.Equal(t, 2, gState.totalCount)
	require.Equal(t, float64(7), gState.totalValue)
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
	require.Equal(t, metrics[0]["name"], aggregate.definition.Name)
	valueFloat64 := metrics[0]["total_value"].(float64)
	require.Equal(t, float64(7), valueFloat64)
}
