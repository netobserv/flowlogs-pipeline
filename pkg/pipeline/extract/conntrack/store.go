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

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

const (
	expiryOrder               = utils.OrderID("expiryOrder")
	nextUpdateReportTimeOrder = utils.OrderID("nextUpdateReportTimeOrder")
)

// connectionStore provides both retrieving a connection by its hash and iterating connections sorted by their last
// update time.
type connectionStore struct {
	mom     *utils.MultiOrderedMap
	metrics *metricsType
	// TBD: keep the selectors data here
}

type processConnF func(connection) (shouldDelete, shouldStop bool)

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	// TBD: determine the group of the connection according to the selectors and add it to the correct group.
	// TBD: add a method to `connection` to check if it matches a selector: isMatchSelector().
	err := cs.mom.AddRecord(utils.Key(hashId), conn)
	if err != nil {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	cs.metrics.connStoreLength.Set(float64(cs.mom.Len()))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	record, ok := cs.mom.GetRecord(utils.Key(hashId))
	if !ok {
		return nil, false
	}
	conn := record.(connection)
	return conn, true
}

// TBD: No need to get the new time from outside. can calculate it inside using the selectors data.
func (cs *connectionStore) updateConnectionExpiryTime(hashId uint64, t time.Time) {
	conn, ok := cs.getConnection(hashId)
	if !ok {
		log.Panicf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	conn.setExpiryTime(t)
	// Move to the back of the list
	err := cs.mom.MoveToBack(utils.Key(hashId), expiryOrder)
	if err != nil {
		log.Panicf("BUG. Can't update connection expiry time for hash %x: %v", hashId, err)
		return
	}
}

func (cs *connectionStore) updateNextReportTime(hashId uint64, t time.Time) {
	conn, ok := cs.getConnection(hashId)
	if !ok {
		log.Panicf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	conn.setNextUpdateReportTime(t)
	// Move to the back of the list
	err := cs.mom.MoveToBack(utils.Key(hashId), nextUpdateReportTimeOrder)
	if err != nil {
		log.Panicf("BUG. Can't next report time for hash %x: %v", hashId, err)
		return
	}
}

func (cs *connectionStore) iterateFrontToBack(orderID utils.OrderID, f processConnF) {
	cs.mom.IterateFrontToBack(orderID, func(r utils.Record) (shouldDelete, shouldStop bool) {
		shouldDelete, shouldStop = f(r.(connection))
		return
	})
	cs.metrics.connStoreLength.Set(float64(cs.mom.Len()))
}

func newConnectionStore(metrics *metricsType) *connectionStore {
	return &connectionStore{
		mom:     utils.NewMultiOrderedMap(expiryOrder, nextUpdateReportTimeOrder),
		metrics: metrics,
	}
}
