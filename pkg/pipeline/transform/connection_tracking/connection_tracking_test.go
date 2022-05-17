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

package connection_tracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_InitConnectionTracking(t *testing.T) {
	InitConnectionTracking()
	require.NotNil(t, CT)
}

func Test_AddFlow(t *testing.T) {
	InitConnectionTracking()
	isNew := CT.AddFlow("test")
	require.Equal(t, true, isNew)
	require.Equal(t, CT.mCache.CacheMap["test"].SourceEntry.(cacheInfo).flowlogsCount, 1)

	isNew = CT.AddFlow("test")
	require.Equal(t, false, isNew)
	require.Equal(t, CT.mCache.CacheMap["test"].SourceEntry.(cacheInfo).flowlogsCount, 1)
}

func Test_IsFlowKnown(t *testing.T) {
	InitConnectionTracking()
	_ = CT.AddFlow("test")
	isNew := CT.IsFlowKnown("test")
	require.Equal(t, true, isNew)
	isNew = CT.IsFlowKnown("test_unknown")
	require.Equal(t, false, isNew)
}

func Test_cleanupExpiredEntries(t *testing.T) {
	InitConnectionTracking()
	CT.expiryTime = 1

	_ = CT.AddFlow("test")
	CT.mCache.CleanupExpiredEntries(CT.expiryTime, CT)
	require.Contains(t, CT.mCache.CacheMap, "test")
	time.Sleep(2 * time.Second)
	CT.mCache.CleanupExpiredEntries(CT.expiryTime, CT)
	require.NotContains(t, CT.mCache.CacheMap, "test")
}

func Test_cleanupExpiredEntriesLoop(t *testing.T) {
	expiryTime = 1
	InitConnectionTracking()

	_ = CT.AddFlow("test")
	require.Equal(t, CT.IsFlowKnown("test"), true)
	time.Sleep(2 * time.Second)
	require.Equal(t, CT.IsFlowKnown("test"), false)
}
