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
	"hash"
	"hash/fnv"
	"strconv"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
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
	metrics                      *metricsType
}

func (ct *conntrackImpl) Extract(flowLogs []config.GenericMap) []config.GenericMap {
	log.Debugf("entering Extract conntrack, in = %v", flowLogs)

	var outputRecords []config.GenericMap
	for _, fl := range flowLogs {
		computedHash, err := ComputeHash(fl, ct.config.KeyDefinition, ct.hashProvider())
		if err != nil {
			log.Warningf("skipping flow log %v: %v", fl, err)
			ct.metrics.inputRecords.WithLabelValues("rejected").Inc()
			continue
		}
		conn, exists := ct.connStore.getConnection(computedHash.hashTotal)
		if !exists {
			if (ct.config.MaxConnectionsTracked > 0) && (ct.config.MaxConnectionsTracked <= ct.connStore.mom.Len()) {
				log.Warningf("too many connections; skipping flow log %v: ", fl)
				ct.metrics.inputRecords.WithLabelValues("discarded").Inc()
			} else {
				builder := NewConnBuilder()
				conn = builder.
					Hash(computedHash).
					KeysFrom(fl, ct.config.KeyDefinition).
					Aggregators(ct.aggregators).
					// TBD: Move the following setting of the next update time to addConnection()
					NextUpdateReportTime(ct.clock.Now().Add(ct.config.UpdateConnectionInterval.Duration)).
					Build()
				ct.connStore.addConnection(computedHash.hashTotal, conn)
				ct.updateConnection(conn, fl, computedHash)
				ct.metrics.inputRecords.WithLabelValues("newConnection").Inc()
				if ct.shouldOutputNewConnection {
					record := conn.toGenericMap()
					addHashField(record, computedHash.hashTotal)
					addTypeField(record, api.ConnTrackOutputRecordTypeName("NewConnection"))
					isFirst := conn.markReported()
					addIsFirstField(record, isFirst)
					outputRecords = append(outputRecords, record)
					ct.metrics.outputRecords.WithLabelValues("newConnection").Inc()
				}
			}
		} else {
			ct.updateConnection(conn, fl, computedHash)
			ct.metrics.inputRecords.WithLabelValues("update").Inc()
		}

		if ct.shouldOutputFlowLogs {
			record := fl.Copy()
			addHashField(record, computedHash.hashTotal)
			addTypeField(record, api.ConnTrackOutputRecordTypeName("FlowLog"))
			outputRecords = append(outputRecords, record)
			ct.metrics.outputRecords.WithLabelValues("flowLog").Inc()
		}
	}

	endConnectionRecords := ct.popEndConnections()
	if ct.shouldOutputEndConnection {
		outputRecords = append(outputRecords, endConnectionRecords...)
		ct.metrics.outputRecords.WithLabelValues("endConnection").Add(float64(len(endConnectionRecords)))
	}

	if ct.shouldOutputUpdateConnection {
		updateConnectionRecords := ct.prepareUpdateConnectionRecords()
		outputRecords = append(outputRecords, updateConnectionRecords...)
		ct.metrics.outputRecords.WithLabelValues("updateConnection").Add(float64(len(updateConnectionRecords)))
	}

	return outputRecords
}

func (ct *conntrackImpl) popEndConnections() []config.GenericMap {
	var outputRecords []config.GenericMap
	// TBD: Move the following logic to store.go maybe except for the addXXX() logic
	// Iterate over the connections by their expiry time from old to new.
	ct.connStore.iterateFrontToBack(expiryOrder, func(conn connection) (shouldDelete, shouldStop bool) {
		expiryTime := conn.getExpiryTime()
		if ct.clock.Now().After(expiryTime) {
			// The connection has expired. We want to pop it.
			record := conn.toGenericMap()
			addHashField(record, conn.getHash().hashTotal)
			addTypeField(record, api.ConnTrackOutputRecordTypeName("EndConnection"))
			var isFirst bool
			if ct.shouldOutputEndConnection {
				isFirst = conn.markReported()
			}
			addIsFirstField(record, isFirst)
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

func (ct *conntrackImpl) prepareUpdateConnectionRecords() []config.GenericMap {
	var outputRecords []config.GenericMap
	// TBD: Move the following logic to store.go maybe except for the addXXX() logic
	// Iterate over the connections by their next update report time from old to new.
	ct.connStore.iterateFrontToBack(nextUpdateReportTimeOrder, func(conn connection) (shouldDelete, shouldStop bool) {
		nextUpdate := conn.getNextUpdateReportTime()
		needToReport := ct.clock.Now().After(nextUpdate)
		if needToReport {
			record := conn.toGenericMap()
			addHashField(record, conn.getHash().hashTotal)
			addTypeField(record, api.ConnTrackOutputRecordTypeName("UpdateConnection"))
			var isFirst bool
			if ct.shouldOutputUpdateConnection {
				isFirst = conn.markReported()
			}
			addIsFirstField(record, isFirst)
			outputRecords = append(outputRecords, record)
			// TBD: move the calculation of newNextUpdate into updateNextReportTime()
			newNextUpdate := ct.clock.Now().Add(ct.config.UpdateConnectionInterval.Duration)
			ct.connStore.updateNextReportTime(conn.getHash().hashTotal, newNextUpdate)
			shouldDelete, shouldStop = false, false
		} else {
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
	// TBD: move the calculation of newExpiryTime into updateConnectionExpiryTime()
	newExpiryTime := ct.clock.Now().Add(ct.config.EndConnectionTimeout.Duration)
	ct.connStore.updateConnectionExpiryTime(flowLogHash.hashTotal, newExpiryTime)
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
func NewConnectionTrack(opMetrics *operational.Metrics, params config.StageParam, clock clock.Clock) (extract.Extractor, error) {
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

	metrics := newMetrics(opMetrics)
	conntrack := &conntrackImpl{
		clock: clock,
		// TBD: pass selector data to newConnectionStore()
		connStore:                    newConnectionStore(metrics),
		config:                       cfg,
		hashProvider:                 fnv.New64a,
		aggregators:                  aggregators,
		shouldOutputFlowLogs:         shouldOutputFlowLogs,
		shouldOutputNewConnection:    shouldOutputNewConnection,
		shouldOutputEndConnection:    shouldOutputEndConnection,
		shouldOutputUpdateConnection: shouldOutputUpdateConnection,
		metrics:                      metrics,
	}
	return conntrack, nil
}

func addHashField(record config.GenericMap, hashId uint64) {
	record[api.HashIdFieldName] = strconv.FormatUint(hashId, 16)
}

func addTypeField(record config.GenericMap, recordType string) {
	record[api.RecordTypeFieldName] = recordType
}

func addIsFirstField(record config.GenericMap, isFirst bool) {
	record[api.IsFirstFieldName] = isFirst
}
