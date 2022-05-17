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

package utils

import (
	"container/list"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Functions to manage an LRU cache with an expiry
// When an item expires, allow a callback to allow the specific implementatoin to perform its particular cleanup

type CacheCallback interface {
	Cleanup(entry interface{})
}

type CacheEntry struct {
	key             string
	lastUpdatedTime int64
	e               *list.Element
	SourceEntry     interface{}
}

type TimedCacheMap map[string]*CacheEntry

type TimedCache struct {
	Mu        sync.Mutex
	CacheList *list.List
	CacheMap  TimedCacheMap
}

func (l *TimedCache) GetCacheEntry(key string) (interface{}, bool) {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	cEntry, ok := l.CacheMap[key]
	if ok {
		return cEntry.SourceEntry, ok
	} else {
		return nil, ok
	}
}

func (l *TimedCache) UpdateCacheEntry(key string, entry interface{}) *CacheEntry {
	var cEntry *CacheEntry
	nowInSecs := time.Now().Unix()
	l.Mu.Lock()
	defer l.Mu.Unlock()
	cEntry, ok := l.CacheMap[key]
	if ok {
		// item already exists in cache; update the element and move to end of list
		cEntry.lastUpdatedTime = nowInSecs
		// move to end of list
		l.CacheList.MoveToBack(cEntry.e)
	} else {
		// create new entry for cache
		cEntry = &CacheEntry{
			lastUpdatedTime: nowInSecs,
			key:             key,
			SourceEntry:     entry,
		}
		// place at end of list
		log.Debugf("adding entry = %v", cEntry)
		cEntry.e = l.CacheList.PushBack(cEntry)
		l.CacheMap[key] = cEntry
		log.Debugf("CacheList = %v", l.CacheList)
	}
	return cEntry
}

// CleanupExpiredEntries removes items from cache that were last touched more than expiryTime seconds ago
func (l *TimedCache) CleanupExpiredEntries(expiryTime int64, callback CacheCallback) {
	log.Debugf("entering cleanupExpiredEntries")
	l.Mu.Lock()
	defer l.Mu.Unlock()
	log.Debugf("cache = %v", l.CacheMap)
	log.Debugf("list = %v", l.CacheList)
	nowInSecs := time.Now().Unix()
	expireTime := nowInSecs - expiryTime
	// go through the list until we reach recently used entries
	for {
		listEntry := l.CacheList.Front()
		if listEntry == nil {
			return
		}
		pCacheInfo := listEntry.Value.(*CacheEntry)
		log.Debugf("lastUpdatedTime = %d, expireTime = %d", pCacheInfo.lastUpdatedTime, expireTime)
		log.Debugf("pCacheInfo = %v", pCacheInfo)
		if pCacheInfo.lastUpdatedTime > expireTime {
			// no more expired items
			return
		}
		callback.Cleanup(pCacheInfo.SourceEntry)
		delete(l.CacheMap, pCacheInfo.key)
		l.CacheList.Remove(listEntry)
	}
}

func NewTimedCache() *TimedCache {
	l := &TimedCache{
		CacheList: list.New(),
		CacheMap:  make(TimedCacheMap),
	}
	return l
}
