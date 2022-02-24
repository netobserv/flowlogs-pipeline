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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	OperationSum   = "sum"
	OperationAvg   = "avg"
	OperationMax   = "max"
	OperationMin   = "min"
	OperationCount = "count"
)

type By []string
type Operation string

type Labels map[string]string
type NormalizedValues string

type Aggregate struct {
	Definition Definition
	Groups     map[NormalizedValues]*GroupState
}

type Definition struct {
	Name      string
	By        By
	Operation Operation
	RecordKey string
}

type GroupState struct {
	normalizedValues NormalizedValues
	RecentRawValues  []float64
	value            float64
	count            int
}

func (aggregate Aggregate) LabelsFromEntry(entry config.GenericMap) (Labels, bool) {
	allLabelsFound := true
	labels := Labels{}

	for _, key := range aggregate.Definition.By {
		value, ok := entry[key]
		if !ok {
			allLabelsFound = false
		}
		labels[key] = fmt.Sprintf("%v", value)
	}

	return labels, allLabelsFound
}

func (labels Labels) getNormalizedValues() NormalizedValues {
	var normalizedAsString string

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		normalizedAsString += labels[k] + ","
	}

	if len(normalizedAsString) > 0 {
		normalizedAsString = normalizedAsString[:len(normalizedAsString)-1]
	}

	return NormalizedValues(normalizedAsString)
}

func (aggregate Aggregate) FilterEntry(entry config.GenericMap) (error, NormalizedValues) {
	labels, allLabelsFound := aggregate.LabelsFromEntry(entry)
	if !allLabelsFound {
		return fmt.Errorf("missing keys in entry"), ""
	}

	normalizedValues := labels.getNormalizedValues()
	return nil, normalizedValues
}

func (aggregate Aggregate) UpdateByEntry(entry config.GenericMap, normalizedValues NormalizedValues) error {
	groupState, ok := aggregate.Groups[normalizedValues]
	if !ok {
		groupState = &GroupState{normalizedValues: normalizedValues}
		switch string(aggregate.Definition.Operation) {
		case OperationSum, OperationMax:
			groupState.value = 0
		case OperationMin:
			groupState.value = math.MaxFloat64
		default:
		}
		aggregate.Groups[normalizedValues] = groupState
	}

	// update value
	recordKey := aggregate.Definition.RecordKey
	operation := aggregate.Definition.Operation

	if operation == OperationCount {
		groupState.value = float64(groupState.count + 1)
		groupState.RecentRawValues = append(groupState.RecentRawValues, 1)
	} else {
		if recordKey != "" {
			value, ok := entry[recordKey]
			if ok {
				valueString := fmt.Sprintf("%v", value)
				valueFloat64, _ := strconv.ParseFloat(valueString, 64)
				groupState.RecentRawValues = append(groupState.RecentRawValues, valueFloat64)
				switch operation {
				case OperationSum:
					groupState.value += valueFloat64
				case OperationMax:
					groupState.value = math.Max(groupState.value, valueFloat64)
				case OperationMin:
					groupState.value = math.Min(groupState.value, valueFloat64)
				case OperationAvg:
					groupState.value = (groupState.value*float64(groupState.count) + valueFloat64) / float64(groupState.count+1)
				}
			}
		}
	}

	// update count
	groupState.count += 1

	return nil
}

func (aggregate Aggregate) Evaluate(entries []config.GenericMap) error {
	for _, entry := range entries {
		// filter entries matching labels with aggregates
		err, normalizedValues := aggregate.FilterEntry(entry)
		if err != nil {
			continue
		}

		// update aggregate group by entry
		err = aggregate.UpdateByEntry(entry, normalizedValues)
		if err != nil {
			log.Debugf("UpdateByEntry error %v", err)
			continue
		}
	}

	return nil
}

func (aggregate Aggregate) GetMetrics() []config.GenericMap {
	var metrics []config.GenericMap
	for _, group := range aggregate.Groups {
		metrics = append(metrics, config.GenericMap{
			"name":                               aggregate.Definition.Name,
			"operation":                          aggregate.Definition.Operation,
			"record_key":                         aggregate.Definition.RecordKey,
			"by":                                 strings.Join(aggregate.Definition.By, ","),
			"aggregate":                          string(group.normalizedValues),
			"value":                              fmt.Sprintf("%f", group.value),
			"recentRawValues":                    group.RecentRawValues,
			"count":                              fmt.Sprintf("%d", group.count),
			aggregate.Definition.Name + "_value": fmt.Sprintf("%f", group.value),
			strings.Join(aggregate.Definition.By, "_"): string(group.normalizedValues),
		})
		// Once reported, we reset the raw values accumulation
		group.RecentRawValues = make([]float64, 0)
	}

	return metrics
}
