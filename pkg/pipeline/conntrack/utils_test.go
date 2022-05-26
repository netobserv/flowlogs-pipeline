package conntrack

import "github.com/netobserv/flowlogs-pipeline/pkg/config"

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
