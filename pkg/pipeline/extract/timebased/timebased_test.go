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
			Name:          "Top2_sum",
			IndexKey:      "SrcAddr",
			IndexKeys:     []string{"SrcAddr"},
			OperationType: "sum",
			OperationKey:  "Bytes",
			TopK:          2,
			TimeInterval:  api.Duration{Duration: 1 * time.Second},
		},
		{
			Name:          "Bot2_last",
			IndexKey:      "SrcAddr",
			IndexKeys:     []string{"SrcAddr"},
			OperationType: "last",
			OperationKey:  "Bytes",
			Reversed:      true,
			TimeInterval:  api.Duration{Duration: 3 * time.Second},
		},
		{
			Name:          "Top4_max",
			IndexKey:      "DstAddr",
			IndexKeys:     []string{"DstAddr"},
			OperationType: "max",
			OperationKey:  "Bytes",
			TopK:          4,
			Reversed:      false,
			TimeInterval:  api.Duration{Duration: 1 * time.Second},
		},
	}
	return rules
}

func Test_CreateIndexKeysAndFilters(t *testing.T) {
	rules := getTimebasedRules()
	indexKeyStructs, filters := CreateIndexKeysAndFilters(rules)
	require.Equal(t, 2, len(indexKeyStructs))
	require.Equal(t, 3, len(filters))
	require.Contains(t, indexKeyStructs, "SrcAddr")
	require.Contains(t, indexKeyStructs, "DstAddr")
	require.Equal(t, time.Duration(3*time.Second), indexKeyStructs["SrcAddr"].maxTimeInterval)
	require.Equal(t, time.Duration(1*time.Second), indexKeyStructs["DstAddr"].maxTimeInterval)
	require.Equal(t, filters[0].IndexKeyDataTable, indexKeyStructs["SrcAddr"])
	require.Equal(t, filters[1].IndexKeyDataTable, indexKeyStructs["SrcAddr"])
	require.Equal(t, filters[2].IndexKeyDataTable, indexKeyStructs["DstAddr"])
}

func Test_CreateIndexKeysAndFiltersError(t *testing.T) {
	rules := []api.TimebasedFilterRule{
		{
			Name:          "filter1",
			IndexKey:      "",
			OperationType: "sum",
			OperationKey:  "operationKey1",
			TopK:          2,
			TimeInterval:  api.Duration{Duration: 10 * time.Second},
		},
		{
			Name:          "filter2",
			IndexKeys:     []string{},
			OperationType: "sum",
			OperationKey:  "operationKey2",
			TopK:          2,
			TimeInterval:  api.Duration{Duration: 10 * time.Second},
		},
	}
	indexKeyStructs, filters := CreateIndexKeysAndFilters(rules)
	require.Equal(t, 0, len(indexKeyStructs))
	require.Equal(t, 0, len(filters))
}

func Test_AddAndDeleteEntryToTables(t *testing.T) {
	rules := getTimebasedRules()
	indexKeyStructs, _ := CreateIndexKeysAndFilters(rules)
	entries := test.GetExtractMockEntries2()
	nowInSecs := time.Now()
	for _, entry := range entries {
		AddEntryToTables(indexKeyStructs, entry, nowInSecs)
	}
	require.Equal(t, 4, len(indexKeyStructs["SrcAddr"].dataTableMap))
	require.Equal(t, 1, len(indexKeyStructs["DstAddr"].dataTableMap))
	require.Contains(t, indexKeyStructs["SrcAddr"].dataTableMap, "10.0.0.1")
	require.Contains(t, indexKeyStructs["SrcAddr"].dataTableMap, "10.0.0.2")
	require.Contains(t, indexKeyStructs["SrcAddr"].dataTableMap, "10.0.0.3")
	require.Contains(t, indexKeyStructs["DstAddr"].dataTableMap, "11.0.0.1")

	// wait for timeout and test that items were deleted from table
	fmt.Printf("going to sleep for timeout value \n")
	time.Sleep(2 * time.Second)
	fmt.Printf("after sleep for timeout value \n")
	nowInSecs = time.Now()
	DeleteOldEntriesFromTables(indexKeyStructs, nowInSecs)
	require.Equal(t, 0, indexKeyStructs["DstAddr"].dataTableMap["11.0.0.1"].Len())
	require.Equal(t, 3, indexKeyStructs["SrcAddr"].dataTableMap["10.0.0.1"].Len())
}
