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
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
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

//////////////////////////

// TODO: Does connectionStore deserve a file of its own?

// connectionStore provides both retrieving a connection by its hash and iterating connections sorted by their last
// update time.
type connectionStore struct {
	hash2conn map[uint64]*list.Element
	connList  *list.List
}

type processConnF func(connection) (shouldDelete, shouldStop bool)

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	_, ok := cs.getConnection(hashId)
	if ok {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	e := cs.connList.PushBack(conn)
	cs.hash2conn[hashId] = e
	metrics.connStoreLength.Set(float64(cs.connList.Len()))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	elem, ok := cs.hash2conn[hashId]
	if ok {
		conn := elem.Value.(connection)
		return conn, ok
	}
	return nil, ok
}

func (cs *connectionStore) updateConnectionTime(hashId uint64, t time.Time) {
	elem, ok := cs.hash2conn[hashId]
	if !ok {
		log.Errorf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	elem.Value.(connection).setLastUpdate(t)
	// move to end of list
	cs.connList.MoveToBack(elem)
}

func (cs *connectionStore) iterateOldToNew(f processConnF) {
	// How to remove element from list while iterating the same list in golang
	// https://stackoverflow.com/a/27662823/2749989
	var next *list.Element
	for e := cs.connList.Front(); e != nil; e = next {
		conn := e.Value.(connection)
		next = e.Next()
		shouldDelete, shouldStop := f(conn)
		if shouldDelete {
			delete(cs.hash2conn, conn.getHash().hashTotal)
			cs.connList.Remove(e)
			metrics.connStoreLength.Set(float64(cs.connList.Len()))
		}
		if shouldStop {
			break
		}
	}
}

func newConnectionStore() *connectionStore {
	return &connectionStore{
		hash2conn: make(map[uint64]*list.Element),
		connList:  list.New(),
	}
}

//////////////////////////

type conntrackImpl struct {
	clock                        clock.Clock
	config                       *api.ConnTrack
	hashProvider                 func() hash.Hash64
	connStore                    *connectionStore
	aggregators                  []aggregator
	shouldOutputFlowLogs         bool
	shouldOutputNewConnection    bool
	shouldOutputEndConnection    bool
	shouldOutputUpdateConnection bool
}

func (ct *conntrackImpl) Extract(flowLogs []config.GenericMap) []config.GenericMap {
	log.Debugf("Entering Track")
	log.Debugf("Track none, in = %v", flowLogs)

	var outputRecords []config.GenericMap
	for _, fl := range flowLogs {
		computedHash, err := ComputeHash(fl, ct.config.KeyDefinition, ct.hashProvider())
		if err != nil {
			log.Warningf("skipping flow log %v: %v", fl, err)
			metrics.inputRecords.WithLabelValues("rejected").Inc()
			continue
		}
		conn, exists := ct.connStore.getConnection(computedHash.hashTotal)
		if !exists {
			builder := NewConnBuilder()
			conn = builder.
				Hash(computedHash).
				KeysFrom(fl, ct.config.KeyDefinition).
				Aggregators(ct.aggregators).
				Build()
			ct.connStore.addConnection(computedHash.hashTotal, conn)
			ct.updateConnection(conn, fl, computedHash)
			metrics.inputRecords.WithLabelValues("newConnection").Inc()
			if ct.shouldOutputNewConnection {
				record := conn.toGenericMap()
				addHashField(record, computedHash.hashTotal)
				addTypeField(record, api.ConnTrackOutputRecordTypeName("NewConnection"))
				outputRecords = append(outputRecords, record)
				metrics.outputRecords.WithLabelValues("newConnection").Inc()
			}
		} else {
			ct.updateConnection(conn, fl, computedHash)
			metrics.inputRecords.WithLabelValues("update").Inc()
		}

		if ct.shouldOutputFlowLogs {
			record := fl.Copy()
			addHashField(record, computedHash.hashTotal)
			addTypeField(record, api.ConnTrackOutputRecordTypeName("FlowLog"))
			outputRecords = append(outputRecords, record)
			metrics.outputRecords.WithLabelValues("flowLog").Inc()
		}
	}

	endConnectionRecords := ct.popEndConnections()
	if ct.shouldOutputEndConnection {
		outputRecords = append(outputRecords, endConnectionRecords...)
		metrics.outputRecords.WithLabelValues("endConnection").Add(float64(len(endConnectionRecords)))
	}

	return outputRecords
}

func (ct *conntrackImpl) popEndConnections() []config.GenericMap {
	var outputRecords []config.GenericMap
	ct.connStore.iterateOldToNew(func(conn connection) (shouldDelete, shouldStop bool) {
		expireTime := ct.clock.Now().Add(-ct.config.EndConnectionTimeout.Duration)
		lastUpdate := conn.getLastUpdate()
		if lastUpdate.Before(expireTime) {
			// The last update time of this connection is too old. We want to pop it.
			record := conn.toGenericMap()
			addHashField(record, conn.getHash().hashTotal)
			addTypeField(record, api.ConnTrackOutputRecordTypeName("EndConnection"))
			outputRecords = append(outputRecords, record)
			shouldDelete, shouldStop = true, false
		} else {
			// No more expired connections
			shouldDelete, shouldStop = false, true
		}
		return
	})
	return outputRecords
}

func (ct *conntrackImpl) updateConnection(conn connection, flowLog config.GenericMap, flowLogHash totalHashType) {
	d := ct.getFlowLogDirection(conn, flowLogHash)
	for _, agg := range ct.aggregators {
		agg.update(conn, flowLog, d)
	}
	ct.connStore.updateConnectionTime(flowLogHash.hashTotal, ct.clock.Now())
}

func (ct *conntrackImpl) getFlowLogDirection(conn connection, flowLogHash totalHashType) direction {
	d := dirNA
	if ct.config.KeyDefinition.Hash.FieldGroupARef != "" {
		if conn.getHash().hashA == flowLogHash.hashA {
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
func NewConnectionTrack(params config.StageParam, clock clock.Clock) (extract.Extractor, error) {
	cfg := params.Extract.ConnTrack
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("ConnectionTrack config is invalid: %w", err)
	}

	var aggregators []aggregator
	for _, of := range cfg.OutputFields {
		agg, err := newAggregator(of)
		if err != nil {
			return nil, fmt.Errorf("error creating aggregator: %w", err)
		}
		aggregators = append(aggregators, agg)
	}
	shouldOutputFlowLogs := false
	shouldOutputNewConnection := false
	shouldOutputEndConnection := false
	shouldOutputUpdateConnection := false
	for _, option := range cfg.OutputRecordTypes {
		switch option {
		case api.ConnTrackOutputRecordTypeName("FlowLog"):
			shouldOutputFlowLogs = true
		case api.ConnTrackOutputRecordTypeName("NewConnection"):
			shouldOutputNewConnection = true
		case api.ConnTrackOutputRecordTypeName("EndConnection"):
			shouldOutputEndConnection = true
		case api.ConnTrackOutputRecordTypeName("UpdateConnection"):
			shouldOutputUpdateConnection = true
		default:
			return nil, fmt.Errorf("unknown OutputRecordTypes: %v", option)
		}
	}

	conntrack := &conntrackImpl{
		clock:                        clock,
		connStore:                    newConnectionStore(),
		config:                       cfg,
		hashProvider:                 fnv.New64a,
		aggregators:                  aggregators,
		shouldOutputFlowLogs:         shouldOutputFlowLogs,
		shouldOutputNewConnection:    shouldOutputNewConnection,
		shouldOutputEndConnection:    shouldOutputEndConnection,
		shouldOutputUpdateConnection: shouldOutputUpdateConnection,
	}
	return conntrack, nil
}

func addHashField(record config.GenericMap, hashId uint64) {
	record[api.HashIdFieldName] = strconv.FormatUint(hashId, 16)
}

func addTypeField(record config.GenericMap, recordType string) {
	record[api.RecordTypeFieldName] = recordType
}
