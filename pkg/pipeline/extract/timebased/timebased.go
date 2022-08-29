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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

const (
	OperationSum  = "sum"
	OperationAvg  = "avg"
	OperationMax  = "max"
	OperationMin  = "min"
	OperationLast = "last"
	OperationDiff = "diff"
)

type FilterStruct struct {
	Rule               api.TimebasedFilterRule
	RecordKeyDataTable *RecordKeyTable
	Results            filterOperationResults
	Output             []filterOperationResult
}

type filterOperationResults map[string]*filterOperationResult

type filterOperationResult struct {
	key             string
	operationResult float64
}

type DataTableMap map[string]*list.List

type RecordKeyTable struct {
	maxTimeInterval time.Duration
	dataTableMap    DataTableMap
}

type TableEntry struct {
	timeStamp time.Time
	entry     config.GenericMap
}

// CreateRecordKeysAndFilters creates structures for each RecordKey that appears in the rules.
// Note that the same RecordKey might appear in more than one Rule.
// Connect RecordKey structure to its filters.
// For each RecordKey, we need a table of history to handle the largest TimeInterval.
func CreateRecordKeysAndFilters(rules []api.TimebasedFilterRule) (map[string]*RecordKeyTable, []FilterStruct) {
	tmpRecordKeyStructs := make(map[string]*RecordKeyTable)
	tmpFilters := make([]FilterStruct, 0)
	for _, filterRule := range rules {
		log.Debugf("CreateRecordKeysAndFilters: filterRule = %v", filterRule)
		// verify there is a valid RecordKey
		if filterRule.RecordKey == "" {
			log.Errorf("missing RecordKey for filter %s", filterRule.Name)
			continue
		}
		rStruct, ok := tmpRecordKeyStructs[filterRule.RecordKey]
		if !ok {
			rStruct = &RecordKeyTable{
				maxTimeInterval: filterRule.TimeInterval.Duration,
				dataTableMap:    make(DataTableMap),
			}
			tmpRecordKeyStructs[filterRule.RecordKey] = rStruct
			log.Debugf("new RecordKeyTable: name = %s = %v", filterRule.RecordKey, *rStruct)
		} else {
			if filterRule.TimeInterval.Duration > rStruct.maxTimeInterval {
				rStruct.maxTimeInterval = filterRule.TimeInterval.Duration
			}
		}
		// verify the validity of the Operation field in the filterRule
		switch filterRule.Operation {
		case OperationLast, OperationDiff, OperationAvg, OperationMax, OperationMin, OperationSum:
			// OK; nothing to do
		default:
			log.Errorf("illegal operation type %s", filterRule.Operation)
			continue
		}
		tmpFilter := FilterStruct{
			Rule:               filterRule,
			RecordKeyDataTable: rStruct,
			Results:            make(filterOperationResults),
		}
		log.Debugf("new Rule = %v", tmpFilter)
		tmpFilters = append(tmpFilters, tmpFilter)
	}
	return tmpRecordKeyStructs, tmpFilters
}
