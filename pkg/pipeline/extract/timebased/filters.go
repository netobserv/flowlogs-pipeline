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

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

func (fs *FilterStruct) CalculateResults(nowInSecs int64) {
	log.Debugf("CalculateResults nowInSecs = %d", nowInSecs)
	oldestValidTime := nowInSecs - int64(fs.Rule.TimeInterval)
	for key, l := range fs.RecordKeyDataTable.dataTableMap {
		var valueFloat64 = float64(0)
		switch fs.Rule.Operation {
		case OperationLast:
			// handle empty list
			if l.Len() == 0 {
				continue
			}
			valueFloat64, _ = utils.ConvertToFloat64(l.Back().Value.(*TableEntry).entry[fs.Rule.OperationKey])
		case OperationDiff:
			for e := l.Front(); e != nil; e = e.Next() {
				cEntry := e.Value.(*TableEntry)
				if cEntry.timeStamp < oldestValidTime {
					// entry is out of time range; ignore it
					continue
				}
				first, _ := utils.ConvertToFloat64(e.Value.(*TableEntry).entry[fs.Rule.OperationKey])
				last, _ := utils.ConvertToFloat64(l.Back().Value.(*TableEntry).entry[fs.Rule.OperationKey])
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

func (fs *FilterStruct) CalculateValue(l *list.List, oldestValidTime int64) float64 {
	log.Debugf("CalculateValue nowInSecs = %d", oldestValidTime)
	currentValue := getInitValue(fs.Rule.Operation)
	nItems := 0
	for e := l.Front(); e != nil; e = e.Next() {
		cEntry := e.Value.(*TableEntry)
		if cEntry.timeStamp < oldestValidTime {
			// entry is out of time range; ignore it
			continue
		}
		valueFloat64, _ := utils.ConvertToFloat64(cEntry.entry[fs.Rule.OperationKey])
		nItems++
		switch fs.Rule.Operation {
		case OperationSum, OperationAvg:
			currentValue += valueFloat64
		case OperationMax:
			currentValue = math.Max(currentValue, valueFloat64)
		case OperationMin:
			currentValue = math.Min(currentValue, valueFloat64)
		}
	}
	if fs.Rule.Operation == OperationAvg && nItems > 0 {
		currentValue = currentValue / float64(nItems)
	}
	return currentValue
}

func getInitValue(operation api.FilterOperation) float64 {
	switch operation {
	case OperationSum, OperationAvg, OperationLast, OperationDiff:
		return 0
	case OperationMax:
		return (-math.MaxFloat64)
	case OperationMin:
		return math.MaxFloat64
	default:
		log.Panicf("unkown operation %v", operation)
		return 0
	}
}

func (fs *FilterStruct) ComputeTopkBotk() {
	var output []filterOperationResult
	if fs.Rule.TopK > 0 {
		output = fs.computeTopK(fs.Results)
	} else if fs.Rule.BotK > 0 {
		output = fs.computeBotK(fs.Results)
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
			"record_key":       fs.Rule.RecordKey,
			"operation":        fs.Rule.Operation,
			"operation_key":    fs.Rule.OperationKey,
			"key_value":        result.key,
			"operation_result": result.operationResult,
		}
		output = append(output, t)
	}
	return output
}
