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
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdmissionLabels(t *testing.T) {
	seen := make(map[string]bool)
	for _, labels := range [][]string{
		nil, {""}, {"", ""}, {"a|b", "c"}, {"a", "b|c"},
		{"a", ""}, {"", "a"}, {"\x00a"}, {"", "a", ""},
		{strings.Repeat("é", 128)},
	} {
		key := admissionLabels(labels)
		require.False(t, seen[key], "different label sets must not share an admission key")
		seen[key] = true
		require.Equal(t, key, admissionLabels(append([]string(nil), labels...)))
	}
}

func TestMetricAdmission(t *testing.T) {
	a := newMetricAdmission(2, time.Second)
	vector, otherVector := new(int), new(int)
	mutations := 0
	mutate := func() error { mutations++; return nil }
	update := func(v interface{}, values []string, expected bool) {
		t.Helper()
		admitted, err := a.update(v, values, mutate)
		require.NoError(t, err)
		require.Equal(t, expected, admitted)
	}
	labels := []string{"a|b", "c"}
	update(vector, labels, true)
	labels[0] = "changed"
	update(vector, []string{"a", "b|c"}, true)
	update(vector, []string{"third"}, false)
	update(vector, []string{"a|b", "c"}, true)
	require.Equal(t, 3, mutations)
	require.Len(t, a.entries, 2)
	a.forget(vector)
	require.Empty(t, a.entries)
	update(vector, []string{"same"}, true)
	update(otherVector, []string{"same"}, true)
	require.Len(t, a.entries, 2, "the cap is shared across vectors")
	a.mu.Lock()
	a.expireLocked(time.Now().Add(2 * time.Second))
	a.mu.Unlock()
	require.Empty(t, a.entries)
	update(vector, []string{"new"}, true)
	a.reset()
	require.Empty(t, a.entries)
	require.Equal(t, 0, a.order.Len())
}

func TestMetricAdmissionFailedMutation(t *testing.T) {
	a := newMetricAdmission(1, time.Hour)
	vector := new(int)
	failure := errors.New("invalid labels")
	admitted, err := a.update(vector, []string{"invalid"}, func() error { return failure })
	require.True(t, admitted)
	require.ErrorIs(t, err, failure)
	require.Empty(t, a.entries, "failed child creation must not consume capacity")
	admitted, err = a.update(vector, []string{"valid"}, func() error { return nil })
	require.True(t, admitted)
	require.NoError(t, err)
	lastSeen := a.order.Front().Value.(*admissionEntry).lastSeen
	_, err = a.update(vector, []string{"valid"}, func() error { return failure })
	require.ErrorIs(t, err, failure)
	require.Equal(t, lastSeen, a.order.Front().Value.(*admissionEntry).lastSeen)
}

func TestMetricAdmissionConcurrent(t *testing.T) {
	a := newMetricAdmission(10, time.Hour)
	vector := new(int)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.update(vector, []string{strconv.Itoa(i)}, func() error {
				accepted.Add(1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
			a.cleanup()
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), accepted.Load())
	require.Len(t, a.entries, 10)
	require.Nil(t, newMetricAdmission(0, time.Hour), "unlimited metrics need no admission index")
}
