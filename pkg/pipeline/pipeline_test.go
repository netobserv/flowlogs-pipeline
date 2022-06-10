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
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/grpc"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
      loki:
`

func Test_transformToLoki(t *testing.T) {
	var transformed []config.GenericMap
	input := []config.GenericMap{{"key": "value"}}
	transform, err := transform.NewTransformNone()
	require.NoError(t, err)
	transformed = append(transformed, transform.Transform(input)...)

	v := test.InitConfig(t, yamlConfigNoParams)
	require.NotNil(t, v)
	loki, err := write.NewWriteLoki(config.Parameters[0])
	require.NoError(t, err)
	loki.Write(transformed)
}

const configTemplate = `---
log-level: debug
pipeline:
  - name: ingest1
  - name: decode1
    follows: ingest1
  - name: transform1
    follows: decode1
  - name: writer1
    follows: transform1
parameters:
  - name: ingest1
    ingest:
      type: file
      file:
        filename: ../../hack/examples/ocp-ipfix-flowlogs.json
  - name: decode1
    decode:
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
	test.InitConfig(t, configTemplate)

	mainPipeline, err = NewPipeline()
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()
	// What is there left to check? Check length of saved data of each stage in private structure.
	ingester := mainPipeline.pipelineStages[0].Ingester.(*ingest.IngestFile)
	decoder := mainPipeline.pipelineStages[1].Decoder.(*decode.DecodeJson)
	writer := mainPipeline.pipelineStages[3].Writer.(*write.WriteNone)
	require.Equal(t, len(ingester.PrevRecords), len(decoder.PrevRecords))
	require.Equal(t, len(ingester.PrevRecords), len(writer.PrevRecords))

	// checking that the processing is done for at least the first line of the logs
	require.Equal(t, ingester.PrevRecords[0], decoder.PrevRecords[0])
	// values checked from the first line of the ../../hack/examples/ocp-ipfix-flowlogs.json file
	require.Equal(t, config.GenericMap{
		"flp_bytes":   float64(20800),
		"flp_dstAddr": "10.130.2.2",
		"flp_dstPort": float64(36936),
		"flp_packets": float64(400),
		"flp_srcAddr": "10.130.2.13",
		"flp_srcPort": float64(3100),
	}, writer.PrevRecords[0])
}

func TestGRPCProtobuf(t *testing.T) {
	port, err := test2.FreeTCPPort()
	require.NoError(t, err)
	test.InitConfig(t, fmt.Sprintf(`---
log-level: debug
pipeline:
  - name: ingest1
  - name: decode1
    follows: ingest1
  - name: writer1
    follows: decode1
parameters:
  - name: ingest1
    ingest:
      type: grpc
      grpc:
        port: %d
  - name: decode1
    decode:
      type: protobuf
  - name: writer1
    write:
      type: stdout
      stdout:
        format: json
`, port))

	pipe, err := NewPipeline()
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

	flowSender, err := grpc.ConnectClient(fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer flowSender.Close()

	startTime := time.Now()
	endTime := startTime.Add(7 * time.Second)
	_, err = flowSender.Client().Send(context.Background(), &pbflow.Records{
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
		}},
	})
	require.NoError(t, err)

	scanner := bufio.NewScanner(capturedOut)
	require.True(t, scanner.Scan())
	capturedRecord := map[string]interface{}{}
	bytes := scanner.Bytes()
	require.NoError(t, json.Unmarshal(bytes, &capturedRecord))

	assert.NotZero(t, capturedRecord["TimeReceived"])
	delete(capturedRecord, "TimeReceived")
	assert.EqualValues(t, map[string]interface{}{
		"FlowDirection":   float64(1),
		"Bytes":           float64(456),
		"SrcAddr":         "1.2.3.4",
		"DstAddr":         "5.6.7.8",
		"DstMac":          "11:22:33:44:55:66",
		"SrcMac":          "01:02:03:04:05:06",
		"SrcPort":         float64(23000),
		"DstPort":         float64(443),
		"Etype":           float64(2048),
		"Packets":         float64(123),
		"Proto":           float64(1),
		"TimeFlowStartMs": float64(startTime.UnixMilli()),
		"TimeFlowEndMs":   float64(endTime.UnixMilli()),
		"Interface":       "eth0",
	}, capturedRecord)
}

func BenchmarkPipeline(b *testing.B) {
	logrus.StandardLogger().SetLevel(logrus.ErrorLevel)
	t := &testing.T{}
	test.InitConfig(t, strings.ReplaceAll(configTemplate, "type: file", "type: file_chunks"))
	if t.Failed() {
		b.Fatalf("unexpected error loading config")
	}
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		p, err := NewPipeline()
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		b.StartTimer()
		p.Run()
	}
}
