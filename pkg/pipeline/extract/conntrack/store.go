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
	"fmt"
	"sort"
	"strings"
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
	groups          []*groupType
	hashId2groupIdx map[uint64]int
	metrics         *metricsType
	now             func() time.Time
}

type groupType struct {
	scheduling api.ConnTrackSchedulingGroup
	mom        *utils.MultiOrderedMap
	labelValue string
}

func (cs *connectionStore) getGroupIdx(conn connection) (groupIdx int) {
	for i, group := range cs.groups {
		if conn.isMatchSelector(group.scheduling.Selector) {
			// connection belongs to scheduling group i
			return i
		}
	}
	// Shouldn't get here since the last scheduling group should have a selector that matches any connection.
	log.Errorf("BUG. connection with hash %x doesn't match any selector", conn.getHash().hashTotal)
	lastGroupIdx := len(cs.groups) - 1
	return lastGroupIdx
}

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	groupIdx := cs.getGroupIdx(conn)
	mom := cs.groups[groupIdx].mom

	err := mom.AddRecord(utils.Key(hashId), conn)
	if err != nil {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	cs.hashId2groupIdx[hashId] = groupIdx

	groupLabel := cs.groups[groupIdx].labelValue
	groupLen := cs.groups[groupIdx].mom.Len()
	cs.metrics.connStoreLength.WithLabelValues(groupLabel).Set(float64(groupLen))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	groupIdx := cs.hashId2groupIdx[hashId]
	mom := cs.groups[groupIdx].mom

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
	mom := cs.groups[groupIdx].mom
	timeout := cs.groups[groupIdx].scheduling.EndConnectionTimeout.Duration
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
	mom := cs.groups[groupIdx].mom
	timeout := cs.groups[groupIdx].scheduling.UpdateConnectionInterval.Duration
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
	// Iterate over the connections by scheduling groups.
	// In each scheduling group iterate over them by their expiry time from old to new.
	var poppedConnections []connection
	for _, group := range cs.groups {
		group.mom.IterateFrontToBack(expiryOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
			conn := r.(connection)
			expiryTime := conn.getExpiryTime()
			if cs.now().After(expiryTime) {
				// The connection has expired. We want to pop it.
				poppedConnections = append(poppedConnections, conn)
				shouldDelete, shouldStop = true, false
				delete(cs.hashId2groupIdx, conn.getHash().hashTotal)
			} else {
				// No more expired connections
				shouldDelete, shouldStop = false, true
			}
			return
		})
		groupLabel := group.labelValue
		groupLen := group.mom.Len()
		cs.metrics.connStoreLength.WithLabelValues(groupLabel).Set(float64(groupLen))
	}
	return poppedConnections
}

func (cs *connectionStore) prepareUpdateConnections() []connection {
	var connections []connection
	// Iterate over the connections by scheduling groups.
	// In each scheduling group iterate over them by their next update report time from old to new.
	for _, group := range cs.groups {
		group.mom.IterateFrontToBack(nextUpdateReportTimeOrder, func(r utils.Record) (shouldDelete, shouldStop bool) {
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

// schedulingGroupToLabelValue returns a string representation of a scheduling group to be used as a Prometheus label
// value.
func schedulingGroupToLabelValue(groupIdx int, group api.ConnTrackSchedulingGroup) string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("%v: ", groupIdx))
	var keys []string
	for k := range group.Selector {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%v, ", k, group.Selector[k]))
	}
	if len(group.Selector) == 0 {
		sb.WriteString("DEFAULT")
	}
	return sb.String()
}

func newConnectionStore(scheduling []api.ConnTrackSchedulingGroup, metrics *metricsType, nowFunc func() time.Time) *connectionStore {
	groups := make([]*groupType, len(scheduling))
	for groupIdx, sg := range scheduling {
		groups[groupIdx] = &groupType{
			scheduling: sg,
			mom:        utils.NewMultiOrderedMap(expiryOrder, nextUpdateReportTimeOrder),
			labelValue: schedulingGroupToLabelValue(groupIdx, sg),
		}
	}

	cs := &connectionStore{
		groups:          groups,
		hashId2groupIdx: map[uint64]int{},
		metrics:         metrics,
		now:             nowFunc,
	}
	return cs
}
