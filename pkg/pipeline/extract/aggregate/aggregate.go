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
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

const (
	OperationSum   = "sum"
	OperationAvg   = "avg"
	OperationMax   = "max"
	OperationMin   = "min"
	OperationCount = "count"
	OperationNop   = "nop"
)

type Labels map[string]string
type NormalizedValues string

type Aggregate struct {
	Definition api.AggregateDefinition
	Groups     map[NormalizedValues]*GroupState
}

type GroupState struct {
	normalizedValues NormalizedValues
	value            interface{} // either float64 or []float64
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
		case OperationSum, OperationMax, OperationCount, OperationAvg:
			groupState.value = float64(0)
		case OperationMin:
			groupState.value = math.MaxFloat64
		case OperationNop:
			groupState.value = []float64{}
		default:
		}
		aggregate.Groups[normalizedValues] = groupState
	}

	// update value
	recordKey := aggregate.Definition.RecordKey
	operation := aggregate.Definition.Operation

	if operation == OperationCount {
		groupState.value = float64(groupState.count + 1)
	} else {
		if recordKey != "" {
			value, ok := entry[recordKey]
			if ok {
				// TODO: remove string parsing
				valueString := fmt.Sprintf("%v", value)
				valueFloat64, err := strconv.ParseFloat(valueString, 64)
				if err != nil {
					return fmt.Errorf("couldn't parse %v as a float. err: %v", valueString, err)
				}
				groupStateFloat, _ := groupState.value.(float64)

				switch operation {
				case OperationSum:
					groupState.value = groupStateFloat + valueFloat64
				case OperationMax:
					groupState.value = math.Max(groupStateFloat, valueFloat64)
				case OperationMin:
					groupState.value = math.Min(groupStateFloat, valueFloat64)
				case OperationAvg:
					groupState.value = (groupStateFloat*float64(groupState.count) + valueFloat64) / float64(groupState.count+1)
				case OperationNop:
					groupStateFloatSlice, ok := groupState.value.([]float64)
					if !ok {
						return fmt.Errorf("couldn't parse %v as a []float", groupState.value)
					}
					groupState.value = append(groupStateFloatSlice, valueFloat64)
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
			log.Errorf("UpdateByEntry error %v", err)
			continue
		}
	}

	return nil
}

func (aggregate Aggregate) GetMetrics() []config.GenericMap {
	var metrics []config.GenericMap
	for _, group := range aggregate.Groups {
		byFieldName := strings.Join(aggregate.Definition.By, "_")
		aggregateFieldName := aggregate.Definition.Name + "_value"
		m := config.GenericMap{
			"name":             aggregate.Definition.Name,
			"operation":        aggregate.Definition.Operation,
			"record_key":       aggregate.Definition.RecordKey,
			"by":               strings.Join(aggregate.Definition.By, ","),
			"aggregate":        string(group.normalizedValues),
			"value":            group.value,
			"count":            fmt.Sprintf("%d", group.count),
			aggregateFieldName: group.value,
			byFieldName:        string(group.normalizedValues),
		}
		if aggregate.Definition.Operation == OperationNop {
			m["recent_value_count"] = len(group.value.([]float64))
			// Once reported, we reset the value accumulation
			group.value = []float64{}
		}
		metrics = append(metrics, m)
	}

	return metrics
}
