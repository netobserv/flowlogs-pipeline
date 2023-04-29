package decode

import (
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDecodeProtobuf(t *testing.T) {
	decoder := Protobuf{}

	someTime := time.Now()
	flow := pbflow.Record{
		Interface:     "eth0",
		EthProtocol:   2048,
		Bytes:         456,
		Packets:       123,
		Direction:     pbflow.Direction_EGRESS,
		TimeFlowStart: timestamppb.New(someTime),
		TimeFlowEnd:   timestamppb.New(someTime),
		Network: &pbflow.Network{
			SrcAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
			},
			DstAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
			},
		},
		DataLink: &pbflow.DataLink{
			DstMac: 0x112233445566,
			SrcMac: 0x010203040506,
		},
		Transport: &pbflow.Transport{
			Protocol: 1,
			SrcPort:  23000,
			DstPort:  443,
		},
		AgentIp: &pbflow.IP{
			IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x0a090807},
		},
		Flags: 0x100,
		Icmp: &pbflow.Icmp{
			IcmpType: 8,
			IcmpCode: 0,
		},
		TcpDropBytes:   100,
		TcpDropPackets: 10,
		TcpDropFlags:   0x200,
		TcpDropState:   6,
		TcpDropCause:   5,
	}
	rawPB, err := proto.Marshal(&flow)
	require.NoError(t, err)

	out, err := decoder.Decode(rawPB)
	require.NoError(t, err)
	assert.NotZero(t, out["TimeReceived"])
	delete(out, "TimeReceived")
	assert.Equal(t, config.GenericMap{
		"FlowDirection":   1,
		"Bytes":           uint64(456),
		"SrcAddr":         "1.2.3.4",
		"DstAddr":         "5.6.7.8",
		"DstMac":          "11:22:33:44:55:66",
		"SrcMac":          "01:02:03:04:05:06",
		"SrcPort":         uint32(23000),
		"DstPort":         uint32(443),
		"Duplicate":       false,
		"Etype":           uint32(2048),
		"Packets":         uint64(123),
		"Proto":           uint32(1),
		"TimeFlowStartMs": someTime.UnixMilli(),
		"TimeFlowEndMs":   someTime.UnixMilli(),
		"Interface":       "eth0",
		"AgentIP":         "10.9.8.7",
		"Flags":           uint32(0x100),
		"IcmpType":        uint32(8),
		"IcmpCode":        uint32(0),
		"TcpDropBytes":    uint64(100),
		"TcpDropPackets":  uint64(10),
		"TcpDropFlags":    uint32(0x200),
		"TcpDropState":    "TCP_CLOSE",
		"TcpDropCause":    "SKB_DROP_REASON_TCP_CSUM",
	}, out)
}

func TestPBFlowToMap(t *testing.T) {
	someTime := time.Now()
	flow := &pbflow.Record{
		Interface:     "eth0",
		EthProtocol:   2048,
		Bytes:         456,
		Packets:       123,
		Direction:     pbflow.Direction_EGRESS,
		TimeFlowStart: timestamppb.New(someTime),
		TimeFlowEnd:   timestamppb.New(someTime),
		Duplicate:     true,
		Network: &pbflow.Network{
			SrcAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
			},
			DstAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
			},
		},
		DataLink: &pbflow.DataLink{
			DstMac: 0x112233445566,
			SrcMac: 0x010203040506,
		},
		Transport: &pbflow.Transport{
			Protocol: 1,
			SrcPort:  23000,
			DstPort:  443,
		},
		AgentIp: &pbflow.IP{
			IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x0a090807},
		},
		Flags: 0x100,
		Icmp: &pbflow.Icmp{
			IcmpType: 10,
			IcmpCode: 11,
		},
		TcpDropBytes:   200,
		TcpDropPackets: 20,
		TcpDropFlags:   0x100,
		TcpDropState:   1,
		TcpDropCause:   4,
	}

	out := PBFlowToMap(flow)
	assert.NotZero(t, out["TimeReceived"])
	delete(out, "TimeReceived")
	assert.Equal(t, config.GenericMap{
		"FlowDirection":   1,
		"Bytes":           uint64(456),
		"SrcAddr":         "1.2.3.4",
		"DstAddr":         "5.6.7.8",
		"DstMac":          "11:22:33:44:55:66",
		"SrcMac":          "01:02:03:04:05:06",
		"SrcPort":         uint32(23000),
		"DstPort":         uint32(443),
		"Duplicate":       true,
		"Etype":           uint32(2048),
		"Packets":         uint64(123),
		"Proto":           uint32(1),
		"TimeFlowStartMs": someTime.UnixMilli(),
		"TimeFlowEndMs":   someTime.UnixMilli(),
		"Interface":       "eth0",
		"AgentIP":         "10.9.8.7",
		"Flags":           uint32(0x100),
		"IcmpType":        uint32(10),
		"IcmpCode":        uint32(11),
		"TcpDropBytes":    uint64(200),
		"TcpDropPackets":  uint64(20),
		"TcpDropFlags":    uint32(0x100),
		"TcpDropState":    "TCP_ESTABLISHED",
		"TcpDropCause":    "SKB_DROP_REASON_PKT_TOO_SMALL",
	}, out)

}
