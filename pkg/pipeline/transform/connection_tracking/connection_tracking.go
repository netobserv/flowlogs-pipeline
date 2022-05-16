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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
)

type cacheInfo struct {
	flowlogsCount int
}

const defaultExpiryTime = 120

type ConnectionTracking struct {
	mCache     *utils.TimedLruCache
	expiryTime int64
}

var (
	CT         ConnectionTracking
	expiryTime = int64(defaultExpiryTime)
)

func (ct ConnectionTracking) IsFlowKnown(flowIDFields string) bool {
	if _, ok := ct.mCache.GetCacheEntry(flowIDFields); ok {
		return true
	}
	return false
}

func (ct ConnectionTracking) AddFlow(flowIDFields string) bool {
	entry := cacheInfo{}
	isNew := true
	if cacheEntry, ok := ct.mCache.GetCacheEntry(flowIDFields); ok {
		entry = cacheEntry.(cacheInfo)
		isNew = false
	}
	entry.flowlogsCount += 1

	ct.mCache.UpdateCacheEntry(flowIDFields, entry)

	return isNew
}

func (ct ConnectionTracking) Cleanup(entry interface{}) {}

func (ct ConnectionTracking) cleanupExpiredEntriesLoop() {
	ticker := time.NewTicker(time.Duration(ct.expiryTime) * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ct.mCache.CleanupExpiredEntries(ct.expiryTime, ct)
			}
		}
	}()
}

func InitConnectionTracking() {
	CT = ConnectionTracking{
		mCache:     utils.NewTimeLruCache(),
		expiryTime: expiryTime,
	}

	CT.cleanupExpiredEntriesLoop()
}
