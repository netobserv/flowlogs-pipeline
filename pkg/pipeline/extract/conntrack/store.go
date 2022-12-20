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
	defaultGroupId            = -1
)

// connectionStore provides both retrieving a connection by its hash and iterating connections sorted by their last
// update time.
type connectionStore struct {
	group2mom    map[int]*utils.MultiOrderedMap
	hashId2group map[uint64]int
	scheduling   []api.ConnTrackSchedulingSelector
	metrics      *metricsType
	now          func() time.Time
}

func (cs *connectionStore) getGroup(conn connection) (groupId int) {
	for i, group := range cs.scheduling {
		if conn.isMatchSelector(group.Selector) {
			// connection belongs to group i
			return i
		}
	}
	return defaultGroupId
}

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	groupId := cs.getGroup(conn)
	mom := cs.group2mom[groupId]

	err := mom.AddRecord(utils.Key(hashId), conn)
	if err != nil {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	cs.hashId2group[hashId] = groupId

	cs.metrics.connStoreLength.Set(float64(len(cs.hashId2group)))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	groupId := cs.hashId2group[hashId]
	mom := cs.group2mom[groupId]

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
	groupId := cs.hashId2group[hashId]
	mom := cs.group2mom[groupId]
	timeout := cs.scheduling[groupId].EndConnectionTimeout.Duration
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
	groupId := cs.hashId2group[hashId]
	mom := cs.group2mom[groupId]
	timeout := cs.scheduling[groupId].UpdateConnectionInterval.Duration
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
	// TBD: Update the following comment
	// Iterate over the connections by their expiry time from old to new.
	var poppedConnections []connection
	for groupId := range cs.scheduling {
		cs.group2mom[groupId].IterateFrontToBack(expiryOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
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
		cs.metrics.connStoreLength.Set(float64(len(cs.hashId2group)))
		// TBD: Think of adding labels to connStoreLength metric per group
	}
	return poppedConnections
}

func (cs *connectionStore) prepareUpdateConnections() []connection {
	var connections []connection
	// Iterate over the connections by groups.
	// In each group iterate over them by their next update report time from old to new.
	for groupId := range cs.scheduling {
		cs.group2mom[groupId].IterateFrontToBack(nextUpdateReportTimeOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
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
	for groupId := range scheduling {
		group2mom[groupId] = utils.NewMultiOrderedMap(expiryOrder, nextUpdateReportTimeOrder)
	}
	cs := &connectionStore{
		group2mom:    group2mom,
		hashId2group: map[uint64]int{},
		scheduling:   scheduling,
		metrics:      metrics,
		now:          nowFunc,
	}
	return cs
}
