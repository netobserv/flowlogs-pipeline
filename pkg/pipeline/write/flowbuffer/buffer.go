/*
 * Copyright (C) 2026 NetObserv Authors.
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

package flowbuffer

import (
	"sync"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
)

// entry holds a flow and its sort key (ms).
type entry struct {
	flow      config.GenericMap
	timestamp int64
}

// Ring is a bounded ring buffer of enriched flows with oldest-first eviction.
type Ring struct {
	mu       sync.RWMutex
	buf      []entry
	head     int // next write index
	size     int
	capacity int
	oldestTs int64
	newestTs int64
}

// NewRing creates a ring buffer with the given capacity (minimum 1).
func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{
		buf:      make([]entry, capacity),
		capacity: capacity,
	}
}

// Insert appends a flow, evicting the oldest entry when full.
func (r *Ring) Insert(flow config.GenericMap) {
	ts, ok := schema.FlowTimestampMs(flow)
	if !ok {
		// Still store; use 0 so time-range filters can exclude if needed
		ts = 0
	}
	cp := flow.Copy()

	r.mu.Lock()
	defer r.mu.Unlock()

	var evictedTs int64
	evicted := false
	if r.size == r.capacity {
		evictedTs = r.buf[r.head].timestamp
		evicted = true
	}

	r.buf[r.head] = entry{flow: cp, timestamp: ts}
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}

	if r.size == 1 {
		r.oldestTs = ts
		r.newestTs = ts
		return
	}
	if !evicted {
		if ts < r.oldestTs || r.oldestTs == 0 {
			r.oldestTs = ts
		}
		if ts > r.newestTs {
			r.newestTs = ts
		}
		return
	}
	// Evicted an entry that may have held the window edge — refresh min/max
	if evictedTs == r.oldestTs || evictedTs == r.newestTs || ts < r.oldestTs || ts > r.newestTs {
		r.recomputeWindowLocked()
		return
	}
}

func (r *Ring) recomputeWindowLocked() {
	if r.size == 0 {
		r.oldestTs = 0
		r.newestTs = 0
		return
	}
	var oldest, newest int64
	first := true
	for i := 0; i < r.size; i++ {
		idx := r.head - 1 - i
		if idx < 0 {
			idx += r.capacity
		}
		ts := r.buf[idx].timestamp
		if first {
			oldest, newest = ts, ts
			first = false
			continue
		}
		if ts > 0 && (oldest == 0 || ts < oldest) {
			oldest = ts
		}
		if ts > newest {
			newest = ts
		}
	}
	r.oldestTs = oldest
	r.newestTs = newest
}

// Size returns the current number of stored flows.
func (r *Ring) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Capacity returns the maximum number of flows.
func (r *Ring) Capacity() int {
	return r.capacity
}

// Window returns oldest and newest timestamps (ms) currently in the buffer.
func (r *Ring) Window() (oldest, newest int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.oldestTs, r.newestTs
}

// QueryFilter selects flows by time range and optional field equality filters.
type QueryFilter struct {
	StartMs int64
	EndMs   int64
	Limit   int
	// FieldEquals maps field name → accepted string values (OR within field, AND across fields).
	FieldEquals map[string][]string
}

// QueryResult is the local buffer query response payload.
type QueryResult struct {
	Flows           []config.GenericMap `json:"flows"`
	OldestTimestamp int64               `json:"oldestTimestamp"`
	NewestTimestamp int64               `json:"newestTimestamp"`
	Size            int                 `json:"size"`
	Capacity        int                 `json:"capacity"`
	Truncated       bool                `json:"truncated"`
}

// Query returns matching flows newest-first, applying limit.
func (r *Ring) Query(f QueryFilter) QueryResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := QueryResult{
		Flows:           make([]config.GenericMap, 0),
		OldestTimestamp: r.oldestTs,
		NewestTimestamp: r.newestTs,
		Size:            r.size,
		Capacity:        r.capacity,
	}
	if r.size == 0 {
		return res
	}
	limit := f.Limit
	if limit <= 0 {
		limit = r.size
	}

	// Iterate newest → oldest
	for i := 0; i < r.size; i++ {
		idx := r.head - 1 - i
		if idx < 0 {
			idx += r.capacity
		}
		e := r.buf[idx]
		if f.StartMs > 0 && e.timestamp > 0 && e.timestamp < f.StartMs {
			continue
		}
		if f.EndMs > 0 && e.timestamp > 0 && e.timestamp > f.EndMs {
			continue
		}
		if !matchFilters(e.flow, f.FieldEquals) {
			continue
		}
		if len(res.Flows) >= limit {
			res.Truncated = true
			break
		}
		res.Flows = append(res.Flows, e.flow.Copy())
	}
	return res
}

func matchFilters(flow config.GenericMap, filters map[string][]string) bool {
	if len(filters) == 0 {
		return true
	}
	for field, values := range filters {
		if len(values) == 0 {
			continue
		}
		raw, ok := flow[field]
		if !ok || raw == nil {
			return false
		}
		s := stringify(raw)
		matched := false
		for _, v := range values {
			if s == v {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func stringify(v interface{}) string {
	return utils.ConvertToString(v)
}
