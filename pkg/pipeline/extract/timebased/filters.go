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
	"fmt"
	"math"
	"strconv"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	log "github.com/sirupsen/logrus"
)

func (fs FilterStruct) CalculateResults(nowInSecs int64) {
	log.Debugf("CalculateResults nowInSecs = %d", nowInSecs)
	oldestValidTime := nowInSecs - int64(fs.rule.TimeInterval)
	for key, l := range fs.recordKeyDataTable.dataTable {
		var valueFloat64 = float64(0)
		switch fs.rule.Operation {
		case OperationLast:
			// handle empty list
			if l.Front() == nil {
				continue
			}
			cEntry := l.Back().Value.(*TableEntry)
			valueString := fmt.Sprintf("%v", cEntry.entry[fs.rule.OperationKey])
			valueFloat64, _ = strconv.ParseFloat(valueString, 64)
		case OperationDiff:
			for e := l.Front(); e != nil; e = e.Next() {
				cEntry := e.Value.(*TableEntry)
				if cEntry.timeStamp < oldestValidTime {
					// entry is out of time range; ignore it
					continue
				}
				valueString := fmt.Sprintf("%v", e.Value.(*TableEntry).entry[fs.rule.OperationKey])
				first, _ := strconv.ParseFloat(valueString, 64)
				valueString = fmt.Sprintf("%v", l.Back().Value.(*TableEntry).entry[fs.rule.OperationKey])
				last, _ := strconv.ParseFloat(valueString, 64)
				valueFloat64 = last - first
			}
		default:
			valueFloat64 = fs.CalculateValue(l, oldestValidTime)
		}
		fs.results[key] = filterOperationResult{
			key:             key,
			operationResult: valueFloat64,
		}
	}
	log.Debugf("CalculateResults results = %v", fs.results)
}

func (fs FilterStruct) CalculateValue(l *list.List, oldestValidTime int64) float64 {
	log.Debugf("CalculateValue nowInSecs = %d", oldestValidTime)
	currentValue := getInitValue(fs.rule.Operation)
	nItems := 0
	// TODO: handle case where there are no valid entries
	for e := l.Front(); e != nil; e = e.Next() {
		cEntry := e.Value.(*TableEntry)
		if cEntry.timeStamp < oldestValidTime {
			// entry is out of time range; ignore it
			continue
		}
		valueString := fmt.Sprintf("%v", cEntry.entry[fs.rule.OperationKey])
		valueFloat64, _ := strconv.ParseFloat(valueString, 64)
		nItems++
		switch fs.rule.Operation {
		case OperationSum, OperationAvg:
			currentValue += valueFloat64
		case OperationMax:
			currentValue = math.Max(currentValue, valueFloat64)
		case OperationMin:
			currentValue = math.Min(currentValue, valueFloat64)
		}
	}
	if fs.rule.Operation == OperationAvg && nItems > 0 {
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
