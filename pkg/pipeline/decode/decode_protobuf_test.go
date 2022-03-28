package decode

import (
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDecodePBFlows(t *testing.T) {
	decoder := Protobuf{}

	someTime := time.Now()
	flow := &pbflow.Record{
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
	}

	out := decoder.Decode([]interface{}{&pbflow.Records{Entries: []*pbflow.Record{flow}}})
	require.Len(t, out, 1)
	assert.NotZero(t, out[0]["TimeReceived"])
	delete(out[0], "TimeReceived")
	assert.Equal(t, config.GenericMap{
		"FlowDirection": 1,
		"Bytes":         uint64(456),
		"SrcAddr":       "1.2.3.4",
		"DstAddr":       "5.6.7.8",
		"DstMac":        "11:22:33:44:55:66",
		"SrcMac":        "01:02:03:04:05:06",
		"SrcPort":       uint32(23000),
		"DstPort":       uint32(443),
		"Etype":         uint32(2048),
		"Packets":       uint64(123),
		"Proto":         uint32(1),
		"TimeFlowStart": someTime.Unix(),
		"TimeFlowEnd":   someTime.Unix(),
		"Interface":     "eth0",
	}, out[0])

}
