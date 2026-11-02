/*
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kubernetes

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func counterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, counter.Write(metric))
	return metric.GetCounter().GetValue()
}

func newTestMissStreakTracker(
	t *testing.T,
	threshold uint64,
	ttl time.Duration,
	capacity int,
) (*MissStreakTracker, *clock.Mock, *prometheus.CounterVec) {
	t.Helper()
	clk := clock.NewMock()
	longStreaks := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_kubernetes_enrichment_long_miss_streaks_total",
		Help: "test",
	}, []string{"side"})
	tracker := NewMissStreakTracker(MissStreakTrackerConfig{
		Threshold: threshold,
		TTL:       ttl,
		Capacity:  capacity,
		Clock:     clk,
	}, MissStreakTrackerMetrics{
		LongMissStreaks: longStreaks,
	})
	return tracker, clk, longStreaks
}

func TestMissStreakTrackerThresholdAndEnrichedFlow(t *testing.T) {
	tracker, clk, longStreaks := newTestMissStreakTracker(t, 2, time.Minute, 10)
	address := netip.MustParseAddr("10.128.2.41")

	tracker.Observe(SourceSide, address, false)
	clk.Add(time.Second)
	tracker.Observe(SourceSide, address, false)
	require.Equal(t, float64(0), counterValue(t, longStreaks.WithLabelValues("source")))

	clk.Add(time.Second)
	tracker.Observe(SourceSide, address, false)
	tracker.Observe(SourceSide, address, false)
	require.Equal(t, float64(1), counterValue(t, longStreaks.WithLabelValues("source")))
	require.Len(t, tracker.entries, 1)
	require.Equal(t, uint64(4), tracker.entries[missStreakKey{address: address, side: SourceSide}].count)

	tracker.Observe(SourceSide, address, true)
	require.Empty(t, tracker.entries)

	// A new streak for the same IP is independent and can cross the threshold again.
	for range 3 {
		tracker.Observe(SourceSide, address, false)
	}
	require.Equal(t, float64(2), counterValue(t, longStreaks.WithLabelValues("source")))
}

func TestMissStreakTrackerSeparatesEndpointSides(t *testing.T) {
	tracker, _, longStreaks := newTestMissStreakTracker(t, 1, time.Minute, 10)
	address := netip.MustParseAddr("10.128.2.41")

	tracker.Observe(SourceSide, address, false)
	tracker.Observe(DestinationSide, address, false)
	tracker.Observe(SourceSide, address, false)

	require.Equal(t, float64(1), counterValue(t, longStreaks.WithLabelValues("source")))
	require.Equal(t, float64(0), counterValue(t, longStreaks.WithLabelValues("destination")))
	require.Len(t, tracker.entries, 2)

	tracker.Observe(SourceSide, address, true)
	require.Len(t, tracker.entries, 1)
	require.Contains(t, tracker.entries, missStreakKey{address: address, side: DestinationSide})
}

func TestMissStreakTrackerExpiresInactiveEntries(t *testing.T) {
	tracker, clk, _ := newTestMissStreakTracker(t, 20, 10*time.Minute, 10)
	address := netip.MustParseAddr("10.128.2.41")

	tracker.Observe(SourceSide, address, false)
	clk.Add(10 * time.Minute)
	tracker.Observe(SourceSide, address, true)

	require.Empty(t, tracker.entries)
}

func TestMissStreakTrackerRejectsNewEntryAtCapacityAndRateLimitsWarning(t *testing.T) {
	tracker, clk, _ := newTestMissStreakTracker(t, 20, time.Hour, 2)
	first := netip.MustParseAddr("10.128.2.1")
	second := netip.MustParseAddr("10.128.2.2")
	third := netip.MustParseAddr("10.128.2.3")

	tracker.Observe(SourceSide, first, false)
	clk.Add(time.Second)
	tracker.Observe(SourceSide, second, false)
	clk.Add(time.Second)
	thirdKey := missStreakKey{address: third, side: SourceSide}
	events := tracker.update(thirdKey, false, clk.Now().UnixNano())

	require.True(t, events.capacityWarning)
	require.Len(t, tracker.entries, 2)
	require.Contains(t, tracker.entries, missStreakKey{address: first, side: SourceSide})
	require.Contains(t, tracker.entries, missStreakKey{address: second, side: SourceSide})
	require.NotContains(t, tracker.entries, thirdKey)

	events = tracker.update(thirdKey, false, clk.Now().UnixNano())
	require.False(t, events.capacityWarning)
	clk.Add(capacityWarningInterval)
	events = tracker.update(thirdKey, false, clk.Now().UnixNano())
	require.True(t, events.capacityWarning)
}

func TestMissStreakTrackerConcurrentObservations(t *testing.T) {
	tracker, _, longStreaks := newTestMissStreakTracker(t, 20, time.Minute, 10)
	address := netip.MustParseAddr("10.128.2.41")
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Observe(SourceSide, address, false)
		}()
	}
	wg.Wait()

	require.Equal(t, float64(1), counterValue(t, longStreaks.WithLabelValues("source")))
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	require.Equal(t, uint64(100), tracker.entries[missStreakKey{address: address, side: SourceSide}].count)
}

func TestMissStreakTrackerDefaults(t *testing.T) {
	tracker := NewMissStreakTracker(MissStreakTrackerConfig{}, MissStreakTrackerMetrics{})
	require.Equal(t, DefaultMissStreakThreshold, tracker.threshold)
	require.Equal(t, DefaultMissStreakTTL.Nanoseconds(), tracker.ttl)
	require.Equal(t, DefaultMissStreakCapacity, tracker.capacity)
}
