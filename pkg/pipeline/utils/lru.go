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

type LruCallback interface {
	Cleanup(entry interface{})
}

type LruCacheEntry struct {
	key             string
	lastUpdatedTime int64
	e               *list.Element
	sourceEntry     interface{}
}

type LruCacheMap map[string]*LruCacheEntry

type TimedLruCache struct {
	Mu           sync.Mutex
	LruCacheList *list.List
	LruCacheMap  LruCacheMap
}

func (l *TimedLruCache) GetEntryInCache(key string) (interface{}, bool) {
	cEntry, ok := l.LruCacheMap[key]
	if ok {
		return cEntry.sourceEntry, ok
	} else {
		return nil, ok
	}
}

func (l *TimedLruCache) SaveEntryInCache(key string, entry interface{}) *LruCacheEntry {
	var cEntry *LruCacheEntry
	nowInSecs := time.Now().Unix()
	l.Mu.Lock()
	defer l.Mu.Unlock()
	cEntry, ok := l.LruCacheMap[key]
	if ok {
		// item already exists in cache; update the element and move to end of list
		cEntry.lastUpdatedTime = nowInSecs
		// move to end of list
		l.LruCacheList.MoveToBack(cEntry.e)
	} else {
		// create new entry for cache
		cEntry = &LruCacheEntry{
			lastUpdatedTime: nowInSecs,
			key:             key,
			sourceEntry:     entry,
		}
		// place at end of list
		log.Debugf("adding entry = %v", cEntry)
		cEntry.e = l.LruCacheList.PushBack(cEntry)
		l.LruCacheMap[key] = cEntry
		log.Debugf("LruCacheList = %v", l.LruCacheList)
	}
	return cEntry
}

// CleanupExpiredEntries removes items from cache that were last touched more than expiryTime seconds ago
func (l *TimedLruCache) CleanupExpiredEntries(expiryTime int64, callback LruCallback) {
	log.Debugf("entering cleanupExpiredEntries")
	l.Mu.Lock()
	defer l.Mu.Unlock()
	log.Debugf("cache = %v", l.LruCacheMap)
	log.Debugf("list = %v", l.LruCacheList)
	nowInSecs := time.Now().Unix()
	expireTime := nowInSecs - expiryTime
	// go through the list until we reach recently used entries
	for {
		listEntry := l.LruCacheList.Front()
		if listEntry == nil {
			return
		}
		pCacheInfo := listEntry.Value.(*LruCacheEntry)
		log.Debugf("lastUpdatedTime = %d, expireTime = %d", pCacheInfo.lastUpdatedTime, expireTime)
		log.Debugf("pCacheInfo = %v", pCacheInfo)
		if pCacheInfo.lastUpdatedTime > expireTime {
			// no more expired items
			return
		}
		callback.Cleanup(pCacheInfo.sourceEntry)
		delete(l.LruCacheMap, pCacheInfo.key)
		l.LruCacheList.Remove(listEntry)
	}
}

func NewTimeLruCache() *TimedLruCache {
	l := &TimedLruCache{
		LruCacheList: list.New(),
		LruCacheMap:  make(LruCacheMap),
	}
	return l
}
