/*
 * Copyright (C) 2026 Red Hat, Inc.
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

package encode

import (
	"container/list"
	"encoding/binary"
	"strings"
	"sync"
	"time"
)

type admissionKey struct {
	vector interface{}
	labels string
}

type admissionEntry struct {
	key      admissionKey
	lastSeen time.Time
}

// metricAdmission tracks only capped Vec children, without retaining child metrics
// or label maps. The least-recently-updated list makes expiry amortized O(1).
type metricAdmission struct {
	mu      sync.Mutex
	limit   int
	expiry  time.Duration
	entries map[admissionKey]*list.Element
	order   list.List
}

func newMetricAdmission(limit int, expiry time.Duration) *metricAdmission {
	if limit <= 0 {
		return nil
	}
	return &metricAdmission{limit: limit, expiry: expiry, entries: make(map[admissionKey]*list.Element)}
}

// admissionLabels uses length prefixes so separator characters and empty values
// cannot alias different label sets. It copies values into one immutable key.
func admissionLabels(values []string) string {
	size := 0
	for _, value := range values {
		size += len(value) + 1
		for length := len(value); length >= 128; length >>= 7 {
			size++
		}
	}
	var key strings.Builder
	key.Grow(size)
	var prefix [binary.MaxVarintLen64]byte
	for _, value := range values {
		n := binary.PutUvarint(prefix[:], uint64(len(value)))
		key.Write(prefix[:n])
		key.WriteString(value)
	}
	return key.String()
}

// update admits a new child only if there is room, but always allows existing
// children to update. Only successful mutations reserve or refresh a slot.
func (a *metricAdmission) update(vector interface{}, values []string, mutate func() error) (bool, error) {
	key := admissionKey{vector: vector, labels: admissionLabels(values)}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked(time.Now())
	entry, exists := a.entries[key]
	if !exists && len(a.entries) >= a.limit {
		return false, nil
	}
	if err := mutate(); err != nil {
		return true, err
	}
	now := time.Now()
	if exists {
		entry.Value.(*admissionEntry).lastSeen = now
		a.order.MoveToBack(entry)
	} else {
		a.entries[key] = a.order.PushBack(&admissionEntry{key: key, lastSeen: now})
	}
	return true, nil
}

func (a *metricAdmission) expireLocked(now time.Time) {
	if a.expiry <= 0 {
		return
	}
	for first := a.order.Front(); first != nil; first = a.order.Front() {
		entry := first.Value.(*admissionEntry)
		if entry.lastSeen.Add(a.expiry).After(now) {
			break
		}
		delete(a.entries, entry.key)
		a.order.Remove(first)
	}
}

func (a *metricAdmission) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked(time.Now())
}

// forget releases slots when a vector is removed or replaced by configuration.
func (a *metricAdmission) forget(vector interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, entry := range a.entries {
		if key.vector == vector {
			delete(a.entries, key)
			a.order.Remove(entry)
		}
	}
}

func (a *metricAdmission) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = make(map[admissionKey]*list.Element)
	a.order.Init()
}
