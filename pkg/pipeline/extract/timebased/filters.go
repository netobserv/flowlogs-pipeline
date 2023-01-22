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

package timebased

import (
	"container/list"
	"math"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

func (fs *FilterStruct) CalculateResults(nowInSecs time.Time) {
	log.Debugf("CalculateResults nowInSecs = %v", nowInSecs)
	oldestValidTime := nowInSecs.Add(-fs.Rule.TimeInterval.Duration)
	for key, l := range fs.IndexKeyDataTable.dataTableMap {
		var valueFloat64 = float64(0)
		var err error
		switch fs.Rule.OperationType {
		case api.FilterOperationName("FilterOperationLast"):
			// handle empty list
			if l.Len() == 0 {
				continue
			}
			valueFloat64, err = utils.ConvertToFloat64(l.Back().Value.(*TableEntry).entry[fs.Rule.OperationKey])
			if err != nil {
				continue
			}
		case api.FilterOperationName("FilterOperationDiff"):
			for e := l.Front(); e != nil; e = e.Next() {
				cEntry := e.Value.(*TableEntry)
				if cEntry.timeStamp.Before(oldestValidTime) {
					// entry is out of time range; ignore it
					continue
				}
				first, err := utils.ConvertToFloat64(e.Value.(*TableEntry).entry[fs.Rule.OperationKey])
				if err != nil {
					continue
				}
				last, err := utils.ConvertToFloat64(l.Back().Value.(*TableEntry).entry[fs.Rule.OperationKey])
				if err != nil {
					continue
				}
				valueFloat64 = last - first
				break
			}
		default:
			valueFloat64 = fs.CalculateValue(l, oldestValidTime)
		}
		fs.Results[key] = &filterOperationResult{
			key:             key,
			operationResult: valueFloat64,
		}
	}
	log.Debugf("CalculateResults Results = %v", fs.Results)
}

func (fs *FilterStruct) CalculateValue(l *list.List, oldestValidTime time.Time) float64 {
	log.Debugf("CalculateValue nowInSecs = %v", oldestValidTime)
	currentValue := getInitValue(fs.Rule.OperationType)
	nItems := 0
	for e := l.Front(); e != nil; e = e.Next() {
		cEntry := e.Value.(*TableEntry)
		if cEntry.timeStamp.Before(oldestValidTime) {
			// entry is out of time range; ignore it
			continue
		}
		valueFloat64, _ := utils.ConvertToFloat64(cEntry.entry[fs.Rule.OperationKey])
		nItems++
		switch fs.Rule.OperationType {
		case api.FilterOperationName("FilterOperationSum"), api.FilterOperationName("FilterOperationAvg"):
			currentValue += valueFloat64
		case api.FilterOperationName("FilterOperationMax"):
			currentValue = math.Max(currentValue, valueFloat64)
		case api.FilterOperationName("FilterOperationMin"):
			currentValue = math.Min(currentValue, valueFloat64)
		}
	}
	if fs.Rule.OperationType == api.FilterOperationName("FilterOperationAvg") && nItems > 0 {
		currentValue = currentValue / float64(nItems)
	}
	return currentValue
}

func getInitValue(operation string) float64 {
	switch operation {
	case api.FilterOperationName("FilterOperationSum"),
		api.FilterOperationName("FilterOperationAvg"),
		api.FilterOperationName("FilterOperationLast"),
		api.FilterOperationName("FilterOperationDiff"):
		return 0
	case api.FilterOperationName("FilterOperationMax"):
		return (-math.MaxFloat64)
	case api.FilterOperationName("FilterOperationMin"):
		return math.MaxFloat64
	default:
		log.Panicf("unknown operation %v", operation)
		return 0
	}
}

func (fs *FilterStruct) ComputeTopkBotk() {
	var output []filterOperationResult
	if fs.Rule.TopK > 0 {
		if fs.Rule.Reversed {
			output = fs.computeBotK(fs.Results)
		} else {
			output = fs.computeTopK(fs.Results)
		}
	} else {
		// return all Results; convert map to array
		output = make([]filterOperationResult, len(fs.Results))
		i := 0
		for _, item := range fs.Results {
			output[i] = *item
			i++
		}
	}
	fs.Output = output
}

func (fs *FilterStruct) CreateGenericMap() []config.GenericMap {
	output := make([]config.GenericMap, 0)
	for _, result := range fs.Output {
		t := config.GenericMap{
			"name":             fs.Rule.Name,
			"index_key":        fs.Rule.IndexKey,
			"operation":        fs.Rule.OperationType,
			"operation_key":    fs.Rule.OperationKey,
			"key":              result.key,
			"operation_result": result.operationResult,
		}
		t[fs.Rule.IndexKey] = result.key
		log.Debugf("FilterStruct CreateGenericMap: %v", t)
		output = append(output, t)
	}
	log.Debugf("FilterStruct CreateGenericMap: output = %v \n", output)
	return output
}
