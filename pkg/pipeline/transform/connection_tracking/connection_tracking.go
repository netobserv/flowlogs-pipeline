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

// TODO: Delete this package once the connection tracking module is done.

package connection_tracking

import (
	"container/list"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

type cacheKey uint32

type cacheInfo struct {
	key             cacheKey
	flowlogsCount   int
	lastUpdatedTime int64
	listElement     *list.Element
}

const defaultExpiryTime = 120

type ConnectionTracking struct {
	cacheMap  map[cacheKey]cacheInfo
	cacheList *list.List

	expiryTime int64
	mutex      *sync.Mutex
}

var (
	CT         ConnectionTracking
	expiryTime = int64(defaultExpiryTime)
)

func getHashKey(flowIDFields string) cacheKey {
	h := fnv.New32a()
	h.Write([]byte(flowIDFields))
	return cacheKey(h.Sum32())
}

func (ct ConnectionTracking) getCacheEntry(flowIDFields string) (cacheInfo, error) {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	key := getHashKey(flowIDFields)
	if entry, ok := ct.cacheMap[key]; ok {
		return entry, nil
	}

	return cacheInfo{}, fmt.Errorf("not found")
}

func (ct ConnectionTracking) updateCacheEntry(flowIDFields string, entry cacheInfo) {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	key := getHashKey(flowIDFields)
	entry.key = key
	entry.lastUpdatedTime = time.Now().Unix()
	// move element to back of the cache list, or add new entry in the back if needed
	if _, ok := ct.cacheMap[key]; ok {
		ct.cacheList.MoveToBack(entry.listElement)
	} else {
		entry.listElement = ct.cacheList.PushBack(&entry)
	}
	ct.cacheMap[key] = entry
}

func (ct ConnectionTracking) IsFlowKnown(flowIDFields string) bool {
	if _, err := ct.getCacheEntry(flowIDFields); err == nil {
		return true
	}
	return false
}

func (ct ConnectionTracking) AddFlow(flowIDFields string) bool {
	entry := cacheInfo{}
	isNew := true
	if cacheEntry, err := ct.getCacheEntry(flowIDFields); err == nil {
		entry = cacheEntry
		isNew = false
	}
	entry.flowlogsCount += 1

	ct.updateCacheEntry(flowIDFields, entry)

	return isNew
}

func (ct ConnectionTracking) cleanupExpiredEntriesLoop() {
	ticker := time.NewTicker(time.Duration(ct.expiryTime) * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ct.cleanupExpiredEntries()
			}
		}
	}()
}

func (ct ConnectionTracking) cleanupExpiredEntries() {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	nowInSecs := time.Now().Unix()
	expireTime := nowInSecs - ct.expiryTime
	for {
		listEntry := ct.cacheList.Front()
		if listEntry == nil {
			return
		}
		pCacheInfo := listEntry.Value.(*cacheInfo)
		if pCacheInfo.lastUpdatedTime > expireTime {
			return
		}
		delete(ct.cacheMap, pCacheInfo.key)
		ct.cacheList.Remove(listEntry)
	}
}

func InitConnectionTracking() {
	CT = ConnectionTracking{
		cacheMap:   map[cacheKey]cacheInfo{},
		cacheList:  list.New(),
		expiryTime: expiryTime,
		mutex:      &sync.Mutex{},
	}

	CT.cleanupExpiredEntriesLoop()
}
