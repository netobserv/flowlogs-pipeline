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
	rule               api.TimebasedFilterRule
	recordKeyDataTable *RecordKeyTable
	results            filterOperationResults
}

type filterOperationResults map[string]filterOperationResult

type filterOperationResult struct {
	key             string
	operationResult float64
}

type DataTableMap map[string]*list.List

type RecordKeyTable struct {
	maxTimeInterval int
	dataTableMap    DataTableMap
}

type TableEntry struct {
	timeStamp int64
	entry     config.GenericMap
}

// Create structures for each RecordKey that appears in the rules.
// Note that the same RecordKey might appear in more than one rule.
// Connect RecordKey structure to its filters.
// For each RecordKey, we need a table of history to handle the largest TimeInterval.
func CreateRecordKeysAndFilters(rules []api.TimebasedFilterRule) (map[string]*RecordKeyTable, []FilterStruct) {
	tmpRecordKeyStructs := make(map[string]*RecordKeyTable)
	tmpFilters := make([]FilterStruct, len(rules))
	for i, filterRule := range rules {
		// verify there is a valid RecordKey
		if filterRule.RecordKey == "" {
			log.Errorf("missing RecordKey for filter %s", filterRule.Name)
			continue
		}
		rStruct, ok := tmpRecordKeyStructs[filterRule.RecordKey]
		if !ok {
			rStruct = &RecordKeyTable{
				maxTimeInterval: filterRule.TimeInterval,
				dataTableMap:    make(DataTableMap),
			}
			tmpRecordKeyStructs[filterRule.RecordKey] = rStruct
			log.Debugf("new RecordKeyTable: name = %s = %v", filterRule.RecordKey, *rStruct)
		} else {
			if filterRule.TimeInterval > rStruct.maxTimeInterval {
				rStruct.maxTimeInterval = filterRule.TimeInterval
			}
		}
		// TODO: verify the validity of the Operation field in the filterRule
		tmpFilters[i] = FilterStruct{
			rule:               filterRule,
			recordKeyDataTable: rStruct,
			results:            make(filterOperationResults),
		}
		log.Debugf("new rule = %v", tmpFilters[i])
	}
	return tmpRecordKeyStructs, tmpFilters
}
