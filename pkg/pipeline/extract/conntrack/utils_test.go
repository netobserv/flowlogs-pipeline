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
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

func newMockFlowLog(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, direction int, bytes, packets int, duplicate bool) config.GenericMap {
	return config.GenericMap{
		"SrcAddr":       srcIP,
		"SrcPort":       srcPort,
		"DstAddr":       dstIP,
		"DstPort":       dstPort,
		"Proto":         protocol,
		"FlowDirection": direction,
		"Bytes":         bytes,
		"Packets":       packets,
		"Duplicate":     duplicate,
	}
}

type mockRecord struct {
	record config.GenericMap
}

func newMockRecordFromFlowLog(fl config.GenericMap) *mockRecord {
	mock := &mockRecord{
		record: config.GenericMap{},
	}
	for k, v := range fl {
		mock.record[k] = v
	}
	mock.withType("flowLog")
	return mock
}

func newMockRecordConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	mock := &mockRecord{
		record: config.GenericMap{
			"SrcAddr":            srcIP,
			"SrcPort":            srcPort,
			"DstAddr":            dstIP,
			"DstPort":            dstPort,
			"Proto":              protocol,
			"Bytes_AB":           bytesAB,
			"Bytes_BA":           bytesBA,
			"Packets_AB":         packetsAB,
			"Packets_BA":         packetsBA,
			"numFlowLogs":        numFlowLogs,
			api.IsFirstFieldName: false,
		},
	}
	return mock
}

func newMockRecordNewConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	return newMockRecordConnAB(srcIP, srcPort, dstIP, dstPort, protocol, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs).
		withType("newConnection").
		markFirst()

}

func newMockRecordEndConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	return newMockRecordConnAB(srcIP, srcPort, dstIP, dstPort, protocol, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs).
		withType("endConnection")

}

func newMockRecordHeartbeatAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	return newMockRecordConnAB(srcIP, srcPort, dstIP, dstPort, protocol, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs).
		withType("heartbeat")

}

func newMockRecordConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	mock := &mockRecord{
		record: config.GenericMap{
			"SrcAddr":            srcIP,
			"SrcPort":            srcPort,
			"DstAddr":            dstIP,
			"DstPort":            dstPort,
			"Proto":              protocol,
			"Bytes":              bytes,
			"Packets":            packets,
			"numFlowLogs":        numFlowLogs,
			api.IsFirstFieldName: false,
		},
	}
	return mock
}

func newMockRecordNewConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	return newMockRecordConn(srcIP, srcPort, dstIP, dstPort, protocol, bytes, packets, numFlowLogs).
		withType("newConnection").
		markFirst()
}

func newMockRecordEndConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	return newMockRecordConn(srcIP, srcPort, dstIP, dstPort, protocol, bytes, packets, numFlowLogs).
		withType("endConnection")
}

func newMockRecordHeartbeat(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	return newMockRecordConn(srcIP, srcPort, dstIP, dstPort, protocol, bytes, packets, numFlowLogs).
		withType("heartbeat")
}

func (m *mockRecord) withHash(hashStr string) *mockRecord {
	m.record[api.HashIdFieldName] = hashStr
	return m
}

func (m *mockRecord) withType(recordType string) *mockRecord {
	m.record[api.RecordTypeFieldName] = recordType
	return m
}

func (m *mockRecord) markFirst() *mockRecord {
	m.record[api.IsFirstFieldName] = true
	return m
}

func (m *mockRecord) get() config.GenericMap {
	return m.record
}
