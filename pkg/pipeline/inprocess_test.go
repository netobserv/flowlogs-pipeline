package pipeline

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestInProcessFLP(t *testing.T) {
	pipeline := config.NewPresetIngesterPipeline()
	pipeline = pipeline.WriteStdout("writer", api.WriteStdout{Format: "json"})
	cfs := config.ConfigFileStruct{
		Pipeline:   pipeline.GetStages(),
		Parameters: pipeline.GetStageParams(),
	}
	ingester, err := StartFLPInProcess(cfs)
	require.NoError(t, err)
	defer ingester.Close()

	capturedOut, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	// yield thread to allow pipe services correctly start
	time.Sleep(10 * time.Millisecond)

	startTime := time.Now()
	endTime := startTime.Add(7 * time.Second)
	someDuration := endTime.Sub(startTime)

	ingester.Write(&pbflow.Records{
		Entries: []*pbflow.Record{{
			Interface:     "eth0",
			EthProtocol:   2048,
			Bytes:         456,
			Packets:       123,
			Direction:     pbflow.Direction_EGRESS,
			TimeFlowStart: timestamppb.New(startTime),
			TimeFlowEnd:   timestamppb.New(endTime),
			Network: &pbflow.Network{
				SrcAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
				},
				DstAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
				},
				Dscp: 1,
			},
			DataLink: &pbflow.DataLink{
				DstMac: 0x112233445566,
				SrcMac: 0x010203040506,
			},
			Transport: &pbflow.Transport{
				Protocol: 17,
				SrcPort:  23000,
				DstPort:  443,
			},
			AgentIp: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x0a0b0c0d},
			},
			PktDropBytes:           100,
			PktDropPackets:         10,
			PktDropLatestFlags:     1,
			PktDropLatestState:     1,
			PktDropLatestDropCause: 8,
			DnsLatency:             durationpb.New(someDuration),
			DnsId:                  1,
			DnsFlags:               0x80,
			DnsErrno:               0,
			TimeFlowRtt:            durationpb.New(someDuration),
		}},
	})

	scanner := bufio.NewScanner(capturedOut)
	require.True(t, scanner.Scan())
	capturedRecord := map[string]interface{}{}
	bytes := scanner.Bytes()
	require.NoError(t, json.Unmarshal(bytes, &capturedRecord), string(bytes))

	assert.NotZero(t, capturedRecord["TimeReceived"])
	delete(capturedRecord, "TimeReceived")
	assert.EqualValues(t, map[string]interface{}{
		"FlowDirection":          float64(1),
		"Bytes":                  float64(456),
		"SrcAddr":                "1.2.3.4",
		"DstAddr":                "5.6.7.8",
		"Dscp":                   float64(1),
		"DstMac":                 "11:22:33:44:55:66",
		"SrcMac":                 "01:02:03:04:05:06",
		"SrcPort":                float64(23000),
		"DstPort":                float64(443),
		"Duplicate":              false,
		"Etype":                  float64(2048),
		"Packets":                float64(123),
		"Proto":                  float64(17),
		"TimeFlowStartMs":        float64(startTime.UnixMilli()),
		"TimeFlowEndMs":          float64(endTime.UnixMilli()),
		"Interface":              "eth0",
		"AgentIP":                "10.11.12.13",
		"PktDropBytes":           float64(100),
		"PktDropPackets":         float64(10),
		"PktDropLatestFlags":     float64(1),
		"PktDropLatestState":     "TCP_ESTABLISHED",
		"PktDropLatestDropCause": "SKB_DROP_REASON_NETFILTER_DROP",
		"DnsLatencyMs":           float64(someDuration.Milliseconds()),
		"DnsId":                  float64(1),
		"DnsFlags":               float64(0x80),
		"DnsErrno":               float64(0),
		"DnsFlagsResponseCode":   "NoError",
		"TimeFlowRttNs":          float64(someDuration.Nanoseconds()),
	}, capturedRecord)
}
