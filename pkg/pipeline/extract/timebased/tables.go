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

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

func AddEntryToTables(recordKeyStructs map[string]*RecordKeyTable, entry config.GenericMap, nowInSecs int64) {
	for key, recordTable := range recordKeyStructs {
		log.Debugf("ExtractTimebased addEntryToTables: key = %s, recordTable = %v", key, recordTable)
		if val, ok := entry[key]; ok {
			cEntry := &TableEntry{
				timeStamp: nowInSecs,
				entry:     entry,
			}
			// allocate list if it does not yet exist
			if recordTable.dataTableMap[val.(string)] == nil {
				recordTable.dataTableMap[val.(string)] = list.New()
			}
			log.Debugf("ExtractTimebased addEntryToTables: adding to table %s", val)
			AddEntryToTable(cEntry, recordTable.dataTableMap[val.(string)])
		}
	}
}

func AddEntryToTable(cEntry *TableEntry, tableList *list.List) {
	log.Debugf("AddEntryToTable: adding table entry %v", cEntry)
	tableList.PushBack(cEntry)
}

func DeleteOldEntriesFromTables(recordKeyStructs map[string]*RecordKeyTable, nowInSecs int64) {
	for _, recordTable := range recordKeyStructs {
		oldestTime := nowInSecs - int64(recordTable.maxTimeInterval)
		for _, tableMap := range recordTable.dataTableMap {
			for {
				head := tableMap.Front()
				if head == nil {
					break
				}
				tableEntry := head.Value.(*TableEntry)
				if tableEntry.timeStamp < oldestTime {
					tableMap.Remove(head)
					continue
				}
				break
			}
			// TODO: if tableMap is empty, we should clean it up and remove it from recordTable.dataTableMap
		}
	}
}

func PrintTable(l *list.List) {
	fmt.Printf("start PrintTable: \n")
	for e := l.Front(); e != nil; e = e.Next() {
		fmt.Printf("PrintTable: e = %v, Value = %v \n", e, e.Value)
	}
	fmt.Printf("end PrintTable: \n")
}
