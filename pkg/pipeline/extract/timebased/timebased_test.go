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
	"fmt"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func getTimebasedRules() []api.TimebasedFilterRule {
	rules := []api.TimebasedFilterRule{
		{
			Name:         "Top2_sum",
			RecordKey:    "SrcAddr",
			Operation:    "sum",
			OperationKey: "Bytes",
			TopK:         2,
			TimeInterval: time.Duration(1 * time.Second),
		},
		{
			Name:         "Bot2_last",
			RecordKey:    "SrcAddr",
			Operation:    "last",
			OperationKey: "Bytes",
			Reversed:     true,
			TimeInterval: time.Duration(3 * time.Second),
		},
		{
			Name:         "Top4_max",
			RecordKey:    "DstAddr",
			Operation:    "max",
			OperationKey: "Bytes",
			TopK:         4,
			Reversed:     false,
			TimeInterval: time.Duration(1 * time.Second),
		},
	}
	return rules
}

func Test_CreateRecordKeysAndFilters(t *testing.T) {
	rules := getTimebasedRules()
	recordKeyStructs, filters := CreateRecordKeysAndFilters(rules)
	require.Equal(t, 2, len(recordKeyStructs))
	require.Equal(t, 3, len(filters))
	require.Contains(t, recordKeyStructs, "SrcAddr")
	require.Contains(t, recordKeyStructs, "DstAddr")
	require.Equal(t, time.Duration(3*time.Second), recordKeyStructs["SrcAddr"].maxTimeInterval)
	require.Equal(t, time.Duration(1*time.Second), recordKeyStructs["DstAddr"].maxTimeInterval)
	require.Equal(t, filters[0].RecordKeyDataTable, recordKeyStructs["SrcAddr"])
	require.Equal(t, filters[1].RecordKeyDataTable, recordKeyStructs["SrcAddr"])
	require.Equal(t, filters[2].RecordKeyDataTable, recordKeyStructs["DstAddr"])
}

func Test_CreateRecordKeysAndFiltersError(t *testing.T) {
	rules := []api.TimebasedFilterRule{
		{
			Name:         "filter1",
			RecordKey:    "",
			Operation:    "sum",
			OperationKey: "operationKey1",
			TopK:         2,
			TimeInterval: time.Duration(10 * time.Second),
		},
	}
	recordKeyStructs, filters := CreateRecordKeysAndFilters(rules)
	require.Equal(t, 0, len(recordKeyStructs))
	require.Equal(t, 0, len(filters))
}

func Test_AddAndDeleteEntryToTables(t *testing.T) {
	rules := getTimebasedRules()
	recordKeyStructs, _ := CreateRecordKeysAndFilters(rules)
	entries := test.GetExtractMockEntries2()
	nowInSecs := time.Now()
	for _, entry := range entries {
		AddEntryToTables(recordKeyStructs, entry, nowInSecs)
	}
	require.Equal(t, 4, len(recordKeyStructs["SrcAddr"].dataTableMap))
	require.Equal(t, 1, len(recordKeyStructs["DstAddr"].dataTableMap))
	require.Contains(t, recordKeyStructs["SrcAddr"].dataTableMap, "10.0.0.1")
	require.Contains(t, recordKeyStructs["SrcAddr"].dataTableMap, "10.0.0.2")
	require.Contains(t, recordKeyStructs["SrcAddr"].dataTableMap, "10.0.0.3")
	require.Contains(t, recordKeyStructs["DstAddr"].dataTableMap, "11.0.0.1")

	// wait for timeout and test that items were deleted from table
	fmt.Printf("going to sleep for timeout value \n")
	time.Sleep(2 * time.Second)
	fmt.Printf("after sleep for timeout value \n")
	nowInSecs = time.Now()
	DeleteOldEntriesFromTables(recordKeyStructs, nowInSecs)
	require.Equal(t, 0, recordKeyStructs["DstAddr"].dataTableMap["11.0.0.1"].Len())
	require.Equal(t, 3, recordKeyStructs["SrcAddr"].dataTableMap["10.0.0.1"].Len())
}
