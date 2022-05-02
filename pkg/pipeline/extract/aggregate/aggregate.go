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
	"container/list"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

const (
	OperationSum       = "sum"
	OperationAvg       = "avg"
	OperationMax       = "max"
	OperationMin       = "min"
	OperationCount     = "count"
	OperationRawValues = "raw_values"
)

type Labels map[string]string
type NormalizedValues string

type Aggregate struct {
	Definition api.AggregateDefinition
	GroupsMap  map[NormalizedValues]*GroupState
	GroupsList *list.List
	mutex      *sync.Mutex
	expiryTime int64
}

type GroupState struct {
	normalizedValues NormalizedValues
	recentRawValues  []float64
	recentOpValue    float64
	recentCount      int
	totalValue       float64
	totalCount       int
	lastUpdatedTime  int64
	listElement      *list.Element
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

func getInitValue(operation string) float64 {
	switch operation {
	case OperationSum, OperationAvg, OperationMax, OperationCount:
		return 0
	case OperationMin:
		return math.MaxFloat64
	case OperationRawValues:
		// Actually, in OperationRawValues the value is ignored.
		return 0
	default:
		log.Panicf("unkown operation %v", operation)
		return 0
	}
}

func (aggregate Aggregate) UpdateByEntry(entry config.GenericMap, normalizedValues NormalizedValues) error {

	aggregate.mutex.Lock()
	defer aggregate.mutex.Unlock()

	groupState, ok := aggregate.GroupsMap[normalizedValues]
	if !ok {
		groupState = &GroupState{normalizedValues: normalizedValues}
		initVal := getInitValue(string(aggregate.Definition.Operation))
		groupState.totalValue = initVal
		groupState.recentOpValue = initVal
		if aggregate.Definition.Operation == OperationRawValues {
			groupState.recentRawValues = make([]float64, 0)
		}
		aggregate.GroupsMap[normalizedValues] = groupState
		groupState.listElement = aggregate.GroupsList.PushBack(groupState)
	} else {
		aggregate.GroupsList.MoveToBack(groupState.listElement)
	}

	// update value
	recordKey := aggregate.Definition.RecordKey
	operation := aggregate.Definition.Operation

	if operation == OperationCount {
		groupState.totalValue = float64(groupState.totalCount + 1)
		groupState.recentOpValue = float64(groupState.recentCount + 1)
	} else {
		if recordKey != "" {
			value, ok := entry[recordKey]
			if ok {
				valueString := fmt.Sprintf("%v", value)
				valueFloat64, _ := strconv.ParseFloat(valueString, 64)
				switch operation {
				case OperationSum:
					groupState.totalValue += valueFloat64
					groupState.recentOpValue += valueFloat64
				case OperationMax:
					groupState.totalValue = math.Max(groupState.totalValue, valueFloat64)
					groupState.recentOpValue = math.Max(groupState.recentOpValue, valueFloat64)
				case OperationMin:
					groupState.totalValue = math.Min(groupState.totalValue, valueFloat64)
					groupState.recentOpValue = math.Min(groupState.recentOpValue, valueFloat64)
				case OperationAvg:
					groupState.totalValue = (groupState.totalValue*float64(groupState.totalCount) + valueFloat64) / float64(groupState.totalCount+1)
					groupState.recentOpValue = (groupState.recentOpValue*float64(groupState.recentCount) + valueFloat64) / float64(groupState.recentCount+1)
				case OperationRawValues:
					groupState.recentRawValues = append(groupState.recentRawValues, valueFloat64)
				}
			}
		}
	}

	// update count
	groupState.totalCount += 1
	groupState.recentCount += 1
	groupState.lastUpdatedTime = time.Now().Unix()

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

func (aggregate Aggregate) computeTopK(inputMetrics []config.GenericMap) []config.GenericMap {
	nItemsInList := int(0)
	topk := aggregate.Definition.TopK

	outputMetrics := make([]config.GenericMap, topk)
	minRecentCount := inputMetrics[0]["recent_count"].(int)
	indexMin := 0
	log.Debugf("minRecentCount = %d", minRecentCount)

	for index, metric := range inputMetrics {
		// fill output with first topk items
		if nItemsInList < topk {
			outputMetrics[nItemsInList] = metric
			nItemsInList++
			count := metric["recent_count"].(int)
			log.Debugf("item added, count = %d, index = %d", count, index)
			if count < minRecentCount {
				minRecentCount = count
				indexMin = index
				log.Debugf("minRecentCount = %d, index = %d", minRecentCount, index)
			}
			continue
		}

		currentRecentCount := metric["recent_count"].(int)
		if currentRecentCount > minRecentCount {
			// replace item that has minRecentCount
			outputMetrics[indexMin] = metric
			log.Debugf("item added, count = %d", currentRecentCount)
			log.Debugf("replaced index = %d, count = %d", indexMin, minRecentCount)
			// find updated minRecentCount
			newMinRecentCount := currentRecentCount
			for i, m := range outputMetrics {
				count := m["recent_count"].(int)
				if count < newMinRecentCount {
					newMinRecentCount = count
					indexMin = i
				}
			}
			minRecentCount = newMinRecentCount
			log.Debugf("minRecentCount = %d, index = %d", minRecentCount, indexMin)
		}
	}
	return outputMetrics
}

func (aggregate Aggregate) GetMetrics() []config.GenericMap {
	aggregate.mutex.Lock()
	defer aggregate.mutex.Unlock()

	var metrics []config.GenericMap
	for _, group := range aggregate.GroupsMap {
		metrics = append(metrics, config.GenericMap{
			"name":              aggregate.Definition.Name,
			"operation":         aggregate.Definition.Operation,
			"record_key":        aggregate.Definition.RecordKey,
			"by":                strings.Join(aggregate.Definition.By, ","),
			"aggregate":         string(group.normalizedValues),
			"total_value":       fmt.Sprintf("%f", group.totalValue),
			"total_count":       fmt.Sprintf("%d", group.totalCount),
			"recent_raw_values": group.recentRawValues,
			"recent_op_value":   group.recentOpValue,
			"recent_count":      group.recentCount,
			strings.Join(aggregate.Definition.By, "_"): string(group.normalizedValues),
		})
		// Once reported, we reset the recentXXX fields
		if aggregate.Definition.Operation == OperationRawValues {
			group.recentRawValues = make([]float64, 0)
		}
		group.recentCount = 0
		group.recentOpValue = getInitValue(string(aggregate.Definition.Operation))
	}

	if aggregate.Definition.TopK > 0 {
		metrics = aggregate.computeTopK(metrics)
	}

	return metrics
}

func (aggregate Aggregate) cleanupExpiredEntries() {
	nowInSecs := time.Now().Unix()
	expireTime := nowInSecs - aggregate.expiryTime

	for {
		listEntry := aggregate.GroupsList.Front()
		if listEntry == nil {
			return
		}
		pCacheInfo := listEntry.Value.(*GroupState)
		if pCacheInfo.lastUpdatedTime > expireTime {
			return
		}
		delete(aggregate.GroupsMap, pCacheInfo.normalizedValues)
		aggregate.GroupsList.Remove(listEntry)
	}
}
