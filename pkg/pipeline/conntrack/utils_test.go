package conntrack

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

func newMockFlowLog(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets int) config.GenericMap {
	return config.GenericMap{
		"SrcAddr": srcIP,
		"SrcPort": srcPort,
		"DstAddr": dstIP,
		"DstPort": dstPort,
		"Proto":   protocol,
		"Bytes":   bytes,
		"Packets": packets,
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
			"SrcAddr":     srcIP,
			"SrcPort":     srcPort,
			"DstAddr":     dstIP,
			"DstPort":     dstPort,
			"Proto":       protocol,
			"Bytes_AB":    bytesAB,
			"Bytes_BA":    bytesBA,
			"Packets_AB":  packetsAB,
			"Packets_BA":  packetsBA,
			"numFlowLogs": numFlowLogs,
		},
	}
	return mock
}

func newMockRecordNewConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	return newMockRecordConnAB(srcIP, srcPort, dstIP, dstPort, protocol, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs).
		withType("newConnection")

}

func newMockRecordEndConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) *mockRecord {
	return newMockRecordConnAB(srcIP, srcPort, dstIP, dstPort, protocol, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs).
		withType("endConnection")

}

func newMockRecordConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	mock := &mockRecord{
		record: config.GenericMap{
			"SrcAddr":     srcIP,
			"SrcPort":     srcPort,
			"DstAddr":     dstIP,
			"DstPort":     dstPort,
			"Proto":       protocol,
			"Bytes":       bytes,
			"Packets":     packets,
			"numFlowLogs": numFlowLogs,
		},
	}
	return mock
}

func newMockRecordNewConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	return newMockRecordConn(srcIP, srcPort, dstIP, dstPort, protocol, bytes, packets, numFlowLogs).
		withType("newConnection")
}

func newMockRecordEndConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) *mockRecord {
	return newMockRecordConn(srcIP, srcPort, dstIP, dstPort, protocol, bytes, packets, numFlowLogs).
		withType("endConnection")
}

func (m *mockRecord) withHash(hashStr string) *mockRecord {
	m.record[api.HashIdFieldName] = hashStr
	return m
}

func (m *mockRecord) withType(recordType string) *mockRecord {
	m.record[api.RecordTypeFieldName] = recordType
	return m
}

func (m *mockRecord) get() config.GenericMap {
	return m.record
}
