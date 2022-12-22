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

package conntrack

import (
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

const (
	expiryOrder               = utils.OrderID("expiryOrder")
	nextUpdateReportTimeOrder = utils.OrderID("nextUpdateReportTimeOrder")
)

// connectionStore provides means to manage the connections such as retrieving a connection by its hash and organizing
// them in groups sorted by expiry time and next report time.
// This allows efficient retrieval and removal of connections.
type connectionStore struct {
	group2mom       map[int]*utils.MultiOrderedMap
	hashId2groupIdx map[uint64]int
	scheduling      []api.ConnTrackSchedulingSelector
	metrics         *metricsType
	now             func() time.Time
}

func (cs *connectionStore) getGroupIdx(conn connection) (groupIdx int) {
	for i, group := range cs.scheduling {
		if conn.isMatchSelector(group.Selector) {
			// connection belongs to group i
			return i
		}
	}
	// Shouldn't get here since the last group should have a selector that matches any connection.
	log.Errorf("BUG. connection with hash %x doesn't match any selector", conn.getHash().hashTotal)
	lastGroupIdx := len(cs.scheduling) - 1
	return lastGroupIdx
}

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	groupIdx := cs.getGroupIdx(conn)
	mom := cs.group2mom[groupIdx]

	err := mom.AddRecord(utils.Key(hashId), conn)
	if err != nil {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	cs.hashId2groupIdx[hashId] = groupIdx

	cs.metrics.connStoreLength.Set(float64(len(cs.hashId2groupIdx)))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	groupIdx := cs.hashId2groupIdx[hashId]
	mom := cs.group2mom[groupIdx]

	record, ok := mom.GetRecord(utils.Key(hashId))
	if !ok {
		return nil, false
	}
	conn := record.(connection)
	return conn, true
}

func (cs *connectionStore) updateConnectionExpiryTime(hashId uint64) {
	conn, ok := cs.getConnection(hashId)
	if !ok {
		log.Panicf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	groupIdx := cs.hashId2groupIdx[hashId]
	mom := cs.group2mom[groupIdx]
	timeout := cs.scheduling[groupIdx].EndConnectionTimeout.Duration
	newExpiryTime := cs.now().Add(timeout)
	conn.setExpiryTime(newExpiryTime)
	// Move to the back of the list
	err := mom.MoveToBack(utils.Key(hashId), expiryOrder)
	if err != nil {
		log.Panicf("BUG. Can't update connection expiry time for hash %x: %v", hashId, err)
		return
	}
}

func (cs *connectionStore) updateNextReportTime(hashId uint64) {
	conn, ok := cs.getConnection(hashId)
	if !ok {
		log.Panicf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	groupIdx := cs.hashId2groupIdx[hashId]
	mom := cs.group2mom[groupIdx]
	timeout := cs.scheduling[groupIdx].UpdateConnectionInterval.Duration
	newNextUpdateReportTime := cs.now().Add(timeout)
	conn.setNextUpdateReportTime(newNextUpdateReportTime)
	// Move to the back of the list
	err := mom.MoveToBack(utils.Key(hashId), nextUpdateReportTimeOrder)
	if err != nil {
		log.Panicf("BUG. Can't next report time for hash %x: %v", hashId, err)
		return
	}
}

func (cs *connectionStore) popEndConnections() []connection {
	// Iterate over the connections by groups.
	// In each group iterate over them by their expiry time from old to new.
	var poppedConnections []connection
	for groupIdx := range cs.scheduling {
		cs.group2mom[groupIdx].IterateFrontToBack(expiryOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
			conn := r.(connection)
			expiryTime := conn.getExpiryTime()
			if cs.now().After(expiryTime) {
				// The connection has expired. We want to pop it.
				poppedConnections = append(poppedConnections, conn)
				shouldDelete, shouldStop = true, false
			} else {
				// No more expired connections
				shouldDelete, shouldStop = false, true
			}
			return
		})
		cs.metrics.connStoreLength.Set(float64(len(cs.hashId2groupIdx)))
		// TBD: Think of adding labels to connStoreLength metric per group
	}
	return poppedConnections
}

func (cs *connectionStore) prepareUpdateConnections() []connection {
	var connections []connection
	// Iterate over the connections by groups.
	// In each group iterate over them by their next update report time from old to new.
	for groupIdx := range cs.scheduling {
		cs.group2mom[groupIdx].IterateFrontToBack(nextUpdateReportTimeOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
			conn := r.(connection)
			nextUpdate := conn.getNextUpdateReportTime()
			needToReport := cs.now().After(nextUpdate)
			if needToReport {
				connections = append(connections, conn)
				cs.updateNextReportTime(conn.getHash().hashTotal)
				shouldDelete, shouldStop = false, false
			} else {
				shouldDelete, shouldStop = false, true
			}
			return
		})
	}
	return connections
}

func newConnectionStore(scheduling []api.ConnTrackSchedulingSelector, metrics *metricsType, nowFunc func() time.Time) *connectionStore {
	group2mom := map[int]*utils.MultiOrderedMap{}
	for groupIdx := range scheduling {
		group2mom[groupIdx] = utils.NewMultiOrderedMap(expiryOrder, nextUpdateReportTimeOrder)
	}
	cs := &connectionStore{
		group2mom:       group2mom,
		hashId2groupIdx: map[uint64]int{},
		scheduling:      scheduling,
		metrics:         metrics,
		now:             nowFunc,
	}
	return cs
}
