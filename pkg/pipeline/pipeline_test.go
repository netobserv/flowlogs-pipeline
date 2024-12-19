/*
 * Copyright (C) 2019 IBM, Inc.
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

package pipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	test2 "github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	grpc "github.com/netobserv/netobserv-ebpf-agent/pkg/grpc/flow"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var yamlConfigNoParams = `
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki: { url: 'http://loki:3100/' }
`

func Test_transformToLoki(t *testing.T) {
	input := config.GenericMap{"key": "value"}
	transform, err := transform.NewTransformNone()
	require.NoError(t, err)
	transformed, _ := transform.Transform(input)
	v, cfg := test.InitConfig(t, yamlConfigNoParams)
	require.NotNil(t, v)
	loki, err := write.NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)
	loki.Write(transformed)
}

const configTemplate = `---
log-level: debug
pipeline:
  - name: ingest1
  - name: transform1
    follows: ingest1
  - name: writer1
    follows: transform1
parameters:
  - name: ingest1
    ingest:
      type: file
      file:
        filename: ../../hack/examples/ocp-ipfix-flowlogs.json
        decoder:
          type: json
  - name: transform1
    transform:
      type: generic
      generic:
        policy: replace_keys
        rules:
          - input: Bytes
            output: flp_bytes
          - input: DstAddr
            output: flp_dstAddr
          - input: DstPort
            output: flp_dstPort
          - input: Packets
            output: flp_packets
          - input: SrcAddr
            output: flp_srcAddr
          - input: SrcPort
            output: flp_srcPort
  - name: writer1
    write:
      type: none
`

func Test_SimplePipeline(t *testing.T) {
	var mainPipeline *Pipeline
	var err error
	_, cfg := test.InitConfig(t, configTemplate)

	mainPipeline, err = NewPipeline(cfg)
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()
	// What is there left to check? Check length of saved data of each stage in private structure.
	writer := mainPipeline.pipelineStages[2].Writer.(*write.None)

	// values checked from the first line of the ../../hack/examples/ocp-ipfix-flowlogs.json file
	test2.Eventually(t, 30*time.Second, func(t require.TestingT) {
		require.Contains(t, writer.PrevRecords(), config.GenericMap{
			"flp_bytes":   float64(20800),
			"flp_dstAddr": "10.130.2.2",
			"flp_dstPort": float64(36936),
			"flp_packets": float64(400),
			"flp_srcAddr": "10.130.2.13",
			"flp_srcPort": float64(3100),
		})
	})
}

func TestGRPCProtobuf(t *testing.T) {
	port, err := test2.FreeTCPPort()
	require.NoError(t, err)
	_, cfg := test.InitConfig(t, fmt.Sprintf(`---
log-level: debug
pipeline:
  - name: ingest1
  - name: writer1
    follows: ingest1
parameters:
  - name: ingest1
    ingest:
      type: grpc
      grpc:
        port: %d
  - name: writer1
    write:
      type: stdout
      stdout:
        format: json
`, port))

	pipe, err := NewPipeline(cfg)
	require.NoError(t, err)

	capturedOut, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	go pipe.Run()

	// yield thread to allow pipe services correctly start
	time.Sleep(10 * time.Millisecond)

	flowSender, err := grpc.ConnectClient("127.0.0.1", port)
	require.NoError(t, err)
	defer flowSender.Close()

	startTime := time.Now()
	endTime := startTime.Add(7 * time.Second)
	someDuration := endTime.Sub(startTime)
	_, err = flowSender.Client().Send(context.Background(), &pbflow.Records{
		Entries: []*pbflow.Record{{
			Interface: "eth0",
			DupList: []*pbflow.DupMapEntry{
				{
					Interface: "eth0",
					Direction: pbflow.Direction_EGRESS,
					Udn:       "udn-1",
				},
			},
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
			DnsErrno:               22,
			TimeFlowRtt:            durationpb.New(someDuration),
		}},
	})
	require.NoError(t, err)

	scanner := bufio.NewScanner(capturedOut)
	require.True(t, scanner.Scan())
	capturedRecord := map[string]interface{}{}
	bytes := scanner.Bytes()
	require.NoError(t, json.Unmarshal(bytes, &capturedRecord), string(bytes))

	assert.NotZero(t, capturedRecord["TimeReceived"])
	delete(capturedRecord, "TimeReceived")
	assert.EqualValues(t, map[string]interface{}{
		"IfDirections":           []interface{}{float64(1)},
		"Bytes":                  float64(456),
		"SrcAddr":                "1.2.3.4",
		"DstAddr":                "5.6.7.8",
		"Dscp":                   float64(1),
		"DstMac":                 "11:22:33:44:55:66",
		"SrcMac":                 "01:02:03:04:05:06",
		"SrcPort":                float64(23000),
		"DstPort":                float64(443),
		"Etype":                  float64(2048),
		"Packets":                float64(123),
		"Proto":                  float64(17),
		"TimeFlowStartMs":        float64(startTime.UnixMilli()),
		"TimeFlowEndMs":          float64(endTime.UnixMilli()),
		"Interfaces":             []interface{}{"eth0"},
		"Udns":                   []interface{}{"udn-1"},
		"AgentIP":                "10.11.12.13",
		"PktDropBytes":           float64(100),
		"PktDropPackets":         float64(10),
		"PktDropLatestFlags":     float64(1),
		"PktDropLatestState":     "TCP_ESTABLISHED",
		"PktDropLatestDropCause": "SKB_DROP_REASON_NETFILTER_DROP",
		"DnsLatencyMs":           float64(someDuration.Milliseconds()),
		"DnsId":                  float64(1),
		"DnsFlags":               float64(0x80),
		"DnsErrno":               float64(22),
		"DnsFlagsResponseCode":   "NoError",
		"TimeFlowRttNs":          float64(someDuration.Nanoseconds()),
	}, capturedRecord)
}

func BenchmarkPipeline(b *testing.B) {
	logrus.StandardLogger().SetLevel(logrus.ErrorLevel)
	t := &testing.T{}
	_, cfg := test.InitConfig(t, strings.ReplaceAll(configTemplate, "type: file", "type: file_chunks"))
	if t.Failed() {
		b.Fatalf("unexpected error loading config")
	}
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		p, err := NewPipeline(cfg)
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		b.StartTimer()
		p.Run()
	}
}
