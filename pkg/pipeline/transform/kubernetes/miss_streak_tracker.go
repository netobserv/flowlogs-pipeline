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
	"math"
	"net/netip"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	DefaultMissStreakThreshold = uint64(20)
	DefaultMissStreakTTL       = 10 * time.Minute
	DefaultMissStreakCapacity  = 10_000
	DefaultPodSubnetLabel      = "Pods"
	capacityWarningInterval    = time.Minute
)

type EndpointSide uint8

const (
	SourceSide EndpointSide = iota
	DestinationSide
)

func (s EndpointSide) String() string {
	if s == DestinationSide {
		return "destination"
	}
	return "source"
}

type MissStreakTrackerConfig struct {
	Threshold uint64
	TTL       time.Duration
	Capacity  int
	Clock     clock.Clock
}

type MissStreakTrackerMetrics struct {
	LongMissStreaks *prometheus.CounterVec
}

type missStreakKey struct {
	address netip.Addr
	side    EndpointSide
}

type missStreakEntry struct {
	count     uint64
	firstSeen int64
	lastSeen  int64
	reported  bool
}

type missStreakEvents struct {
	thresholdEvent  *missStreakEntry
	resumedEvent    *missStreakEntry
	capacityWarning bool
}

type MissStreakTracker struct {
	mu              sync.Mutex
	entries         map[missStreakKey]missStreakEntry
	threshold       uint64
	ttl             int64
	capacity        int
	clock           clock.Clock
	lastCleanup     int64
	cleanupInterval int64
	capacityWarned  bool
	lastCapacityLog int64
	metrics         MissStreakTrackerMetrics
	log             *logrus.Entry
}

func NewMissStreakTracker(cfg MissStreakTrackerConfig, metrics MissStreakTrackerMetrics) *MissStreakTracker {
	if cfg.Threshold == 0 {
		cfg.Threshold = DefaultMissStreakThreshold
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultMissStreakTTL
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = DefaultMissStreakCapacity
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.New()
	}
	cleanupInterval := min(cfg.TTL, time.Minute)
	return &MissStreakTracker{
		entries:         make(map[missStreakKey]missStreakEntry),
		threshold:       cfg.Threshold,
		ttl:             cfg.TTL.Nanoseconds(),
		capacity:        cfg.Capacity,
		clock:           cfg.Clock,
		cleanupInterval: cleanupInterval.Nanoseconds(),
		metrics:         metrics,
		log:             logrus.WithField("component", "transform.KubernetesEnrichmentMissTracker"),
	}
}

// Observe records whether one Pod endpoint was enriched in the final flow.
func (t *MissStreakTracker) Observe(side EndpointSide, address netip.Addr, enriched bool) {
	if !address.IsValid() {
		return
	}

	now := t.clock.Now().UnixNano()
	address = address.Unmap()
	events := t.update(missStreakKey{address: address, side: side}, enriched, now)

	if events.capacityWarning {
		t.reportCapacityReached(side, address)
	}
	if events.thresholdEvent != nil {
		t.reportThresholdCrossing(side, address, *events.thresholdEvent)
	}
	if events.resumedEvent != nil {
		t.reportEnrichmentResumed(side, address, *events.resumedEvent, now)
	}
}

func (t *MissStreakTracker) update(key missStreakKey, enriched bool, now int64) missStreakEvents {
	t.mu.Lock()
	defer t.mu.Unlock()

	events := missStreakEvents{}
	if t.lastCleanup == 0 || now-t.lastCleanup >= t.cleanupInterval {
		t.removeExpiredLocked(now)
		t.lastCleanup = now
	}

	entry, exists := t.entries[key]
	if exists && now-entry.lastSeen >= t.ttl {
		delete(t.entries, key)
		exists = false
	}

	if enriched {
		t.handleEnrichedLocked(key, entry, exists, &events)
	} else {
		t.handleUnenrichedLocked(key, entry, exists, now, &events)
	}
	return events
}

func (t *MissStreakTracker) handleEnrichedLocked(
	key missStreakKey,
	entry missStreakEntry,
	exists bool,
	events *missStreakEvents,
) {
	if !exists {
		return
	}
	delete(t.entries, key)
	if entry.reported {
		entryCopy := entry
		events.resumedEvent = &entryCopy
	}
}

func (t *MissStreakTracker) handleUnenrichedLocked(
	key missStreakKey,
	entry missStreakEntry,
	exists bool,
	now int64,
	events *missStreakEvents,
) {
	if !exists {
		if len(t.entries) >= t.capacity {
			events.capacityWarning = t.shouldReportCapacityLocked(now)
			return
		}
		entry = missStreakEntry{firstSeen: now}
	}
	if entry.count < math.MaxUint64 {
		entry.count++
	}
	entry.lastSeen = now
	if !entry.reported && entry.count > t.threshold {
		entry.reported = true
		entryCopy := entry
		events.thresholdEvent = &entryCopy
	}
	t.entries[key] = entry
}

func (t *MissStreakTracker) reportThresholdCrossing(
	side EndpointSide,
	address netip.Addr,
	entry missStreakEntry,
) {
	if t.metrics.LongMissStreaks != nil {
		t.metrics.LongMissStreaks.WithLabelValues(side.String()).Inc()
	}
	t.log.WithFields(logrus.Fields{
		"event":           "kubernetes_enrichment_long_miss_streak",
		"side":            side.String(),
		"ip":              address.String(),
		"misses":          entry.count,
		"first_miss_time": time.Unix(0, entry.firstSeen).UTC().Format(time.RFC3339Nano),
		"last_miss_time":  time.Unix(0, entry.lastSeen).UTC().Format(time.RFC3339Nano),
	}).Warn("Kubernetes enrichment miss streak exceeded threshold")
}

func (t *MissStreakTracker) reportEnrichmentResumed(
	side EndpointSide,
	address netip.Addr,
	entry missStreakEntry,
	now int64,
) {
	t.log.WithFields(logrus.Fields{
		"event":              "kubernetes_enrichment_resumed",
		"side":               side.String(),
		"ip":                 address.String(),
		"misses":             entry.count,
		"streak_duration":    time.Duration(entry.lastSeen - entry.firstSeen).String(),
		"first_miss_time":    time.Unix(0, entry.firstSeen).UTC().Format(time.RFC3339Nano),
		"last_miss_time":     time.Unix(0, entry.lastSeen).UTC().Format(time.RFC3339Nano),
		"enriched_flow_time": time.Unix(0, now).UTC().Format(time.RFC3339Nano),
	}).Info("Kubernetes enrichment resumed after a long miss streak on this processor")
}

func (t *MissStreakTracker) reportCapacityReached(side EndpointSide, address netip.Addr) {
	t.log.WithFields(logrus.Fields{
		"event":    "kubernetes_enrichment_miss_tracker_capacity_reached",
		"capacity": t.capacity,
		"side":     side.String(),
		"ip":       address.String(),
	}).Warn("Kubernetes enrichment miss tracker capacity reached; new streak is not being tracked")
}

func (t *MissStreakTracker) shouldReportCapacityLocked(now int64) bool {
	if t.capacityWarned && now-t.lastCapacityLog < capacityWarningInterval.Nanoseconds() {
		return false
	}
	t.capacityWarned = true
	t.lastCapacityLog = now
	return true
}

func (t *MissStreakTracker) removeExpiredLocked(now int64) {
	for key, entry := range t.entries {
		if now-entry.lastSeen >= t.ttl {
			delete(t.entries, key)
		}
	}
}
