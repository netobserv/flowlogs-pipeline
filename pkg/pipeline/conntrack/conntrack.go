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
	"container/list"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

// direction indicates the direction of a flow log in a connection. It's used by aggregators to determine which split
// of the aggregator should be updated, xxx_AB or xxx_BA.
type direction uint8

const (
	dirNA direction = iota
	dirAB
	dirBA
)

type ConnectionTracker interface {
	Track(flowLogs []config.GenericMap) []config.GenericMap
}

//////////////////////////

// TODO: Does connectionStore deserve a file of its own?

// connectionStore provides both retrieving a connection by its hash and iterating connections sorted by their last
// update time.
type connectionStore struct {
	// TODO: should the key of the map be a custom hashStrType instead of string?
	hash2conn map[string]*list.Element
	connList  *list.List
}

type processConnF func(connection) (shouldDelete, shouldStop bool)

func (ct *connectionStore) addConnection(hashStr string, conn connection) {
	_, ok := ct.getConnection(hashStr)
	if ok {
		log.Errorf("BUG. connection with hash %v already exists in store. %v", hashStr, conn)
	}
	e := ct.connList.PushBack(conn)
	ct.hash2conn[hashStr] = e
}

func (ct *connectionStore) getConnection(hashStr string) (connection, bool) {
	elem, ok := ct.hash2conn[hashStr]
	if ok {
		conn := elem.Value.(connection)
		return conn, ok
	}
	return nil, ok
}

func (ct *connectionStore) updateConnectionTime(hashStr string, t time.Time) {
	elem, ok := ct.hash2conn[hashStr]
	if !ok {
		log.Errorf("BUG. connection hash %v doesn't exist", hashStr)
	}
	elem.Value.(connection).setLastUpdateTime(t)
	// move to end of list
	ct.connList.MoveToBack(elem)
}

func (ct *connectionStore) iterateOldToNew(f processConnF) {
	for e := ct.connList.Front(); e != nil; e = e.Next() {
		conn := e.Value.(connection)
		shouldDelete, shouldStop := f(conn)
		if shouldDelete {
			delete(ct.hash2conn, hex.EncodeToString(conn.getHash().hashTotal))
			ct.connList.Remove(e)
		}
		if shouldStop {
			break
		}
	}
}

func newConnectionStore() *connectionStore {
	return &connectionStore{
		hash2conn: make(map[string]*list.Element),
		connList:  list.New(),
	}
}

//////////////////////////

type conntrackImpl struct {
	clock                     clock.Clock
	config                    api.ConnTrack
	hasher                    hash.Hash
	connStore                 *connectionStore
	aggregators               []aggregator
	shouldOutputFlowLogs      bool
	shouldOutputNewConnection bool
	shouldOutputEndConnection bool
}

func (ct *conntrackImpl) Track(flowLogs []config.GenericMap) []config.GenericMap {
	log.Debugf("Entering Track")
	log.Debugf("Track none, in = %v", flowLogs)

	var outputRecords []config.GenericMap
	for _, fl := range flowLogs {
		computedHash, err := ComputeHash(fl, ct.config.KeyDefinition, ct.hasher)
		if err != nil {
			log.Warningf("skipping flow log %v: %v", fl, err)
			continue
		}
		hashStr := hex.EncodeToString(computedHash.hashTotal)
		conn, exists := ct.connStore.getConnection(hashStr)
		if !exists {
			builder := NewConnBuilder()
			conn = builder.
				Hash(computedHash).
				KeysFrom(fl, ct.config.KeyDefinition).
				Aggregators(ct.aggregators).
				Build()
			ct.connStore.addConnection(hashStr, conn)
			ct.updateConnection(conn, fl, computedHash)
			if ct.shouldOutputNewConnection {
				outputRecords = append(outputRecords, conn.toGenericMap())
			}
		} else {
			ct.updateConnection(conn, fl, computedHash)
		}

		if ct.shouldOutputFlowLogs {
			outputRecords = append(outputRecords, fl)
		}
	}

	endConnectionRecords := ct.popEndConnections()
	if ct.shouldOutputEndConnection {
		outputRecords = append(outputRecords, endConnectionRecords...)
	}

	return outputRecords
}

func (ct *conntrackImpl) popEndConnections() []config.GenericMap {
	var outputRecords []config.GenericMap
	ct.connStore.iterateOldToNew(func(conn connection) (shouldDelete, shouldStop bool) {
		expireTime := ct.clock.Now().Add(-ct.config.EndConnectionTimeout)
		lastUpdateTime := conn.getLastUpdateTime()
		if lastUpdateTime.Before(expireTime) {
			// The last update time of this connection is too old. We want to pop it.
			outputRecords = append(outputRecords, conn.toGenericMap())
			shouldDelete, shouldStop = true, false
		} else {
			// No more expired connections
			shouldDelete, shouldStop = false, true
		}
		return
	})
	return outputRecords
}

func (ct *conntrackImpl) updateConnection(conn connection, flowLog config.GenericMap, flowLogHash *totalHashType) {
	d := ct.getFlowLogDirection(conn, flowLogHash)
	for _, agg := range ct.aggregators {
		agg.update(conn, flowLog, d)
	}
	ct.connStore.updateConnectionTime(hex.EncodeToString(flowLogHash.hashTotal), ct.clock.Now())
}

func (ct *conntrackImpl) getFlowLogDirection(conn connection, flowLogHash *totalHashType) direction {
	d := dirNA
	if ct.config.KeyDefinition.Hash.FieldGroupARef != "" {
		if areHashEqual(conn.getHash().hashA, flowLogHash.hashA) {
			// A -> B
			d = dirAB
		} else {
			// B -> A
			d = dirBA
		}
	}
	return d
}

// NewConnectionTrack creates a new connection track instance
func NewConnectionTrack(config api.ConnTrack, clock clock.Clock) (ConnectionTracker, error) {
	var aggregators []aggregator
	for _, of := range config.OutputFields {
		agg, err := newAggregator(of)
		if err != nil {
			return nil, fmt.Errorf("error creating aggregator: %w", err)
		}
		aggregators = append(aggregators, agg)
	}
	shouldOutputFlowLogs := false
	shouldOutputNewConnection := false
	shouldOutputEndConnection := false
	for _, option := range config.OutputRecordTypes {
		switch option {
		case api.ConnTrackOutputRecordTypeName("FlowLog"):
			shouldOutputFlowLogs = true
		case api.ConnTrackOutputRecordTypeName("NewConnection"):
			shouldOutputNewConnection = true
		case api.ConnTrackOutputRecordTypeName("EndConnection"):
			shouldOutputEndConnection = true
		default:
			return nil, fmt.Errorf("unknown OutputRecordTypes: %v", option)
		}
	}

	conntrack := &conntrackImpl{
		clock:                     clock,
		connStore:                 newConnectionStore(),
		config:                    config,
		hasher:                    fnv.New32a(),
		aggregators:               aggregators,
		shouldOutputFlowLogs:      shouldOutputFlowLogs,
		shouldOutputNewConnection: shouldOutputNewConnection,
		shouldOutputEndConnection: shouldOutputEndConnection,
	}
	return conntrack, nil
}
