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

package pipeline

import (
	"bufio"
	"os"
	"sync/atomic"
	"testing"
	"time"

	test2 "github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testConfigConntrack = `---
pipeline:
  - name: ingest_fake
  - follows: ingest_fake
    name: conntrack
  - follows: conntrack
    name: write_fake
parameters:
  - ingest:
      type: fake
    name: ingest_fake
  - name: conntrack
    extract:
      type: conntrack
      conntrack:
        scheduling:
          - selector: {}
            endConnectionTimeout: 1s
            heartbeatInterval: 10s
            terminatingTimeout: 5s
        outputRecordTypes:
          - newConnection
          - flowLog
          - endConnection
        keyDefinition:
          fieldGroups:
            - name: src
              fields:
                - SrcAddr
                - SrcPort
            - name: dst
              fields:
                - DstAddr
                - DstPort
            - name: protocol
              fields:
                - Proto
          hash:
            fieldGroupRefs: [protocol]
            fieldGroupARef: src 
            fieldGroupBRef: dst 
        outputFields:
          - name: Bytes
            operation: sum
            SplitAB: true
          - name: Packets
            operation: sum
            SplitAB: true
          - name: numFlowLogs
            operation: count
          - name: TimeFlowStart
            operation: min
            input: TimeReceived
          - name: TimeFlowEnd
            operation: max
            input: TimeReceived
  - name: write_fake
    write:
      type: fake
`

func TestConnTrack(t *testing.T) {
	// This test runs a 3 stage pipeline (Ingester -> ConnTrack -> Write), ingests a file and tests that an end
	// connection record with specific values was written.
	var mainPipeline *Pipeline
	var err error
	v, cfg := test.InitConfig(t, testConfigConntrack)
	require.NotNil(t, v)
	cfg.PerfSettings.BatcherMaxLen = 200_000
	cfg.PerfSettings.BatcherTimeout = 2 * time.Second

	mainPipeline, err = NewPipeline(cfg)
	require.NoError(t, err)

	go mainPipeline.Run()

	ingest := mainPipeline.pipelineStages[0].Ingester.(*ingest.Fake)
	in := ingest.In
	writer := mainPipeline.pipelineStages[2].Writer.(*write.Fake)

	sentLines := ingestFile(t, in, "../../hack/examples/ocp-ipfix-flowlogs.json")

	// wait for all the lines to be ingested
	test2.Eventually(t, 15*time.Second, func(t require.TestingT) {
		count := atomic.LoadInt64(&ingest.Count)
		require.EqualValues(t, count, sentLines, "sent: %d. got: %d", sentLines, count)
	}, test2.Interval(10*time.Millisecond))

	// Wait a moment to make the connections expired
	time.Sleep(5 * time.Second)

	// Send something to the pipeline to allow the connection tracking output end connection records
	in <- config.GenericMap{"DstAddr": "1.2.3.4"}

	// Verify that the output records contain an expected end connection record.
	expected := config.GenericMap{
		"Bytes_AB":      41_600.0,
		"Bytes_BA":      5_004_000.0,
		"DstAddr":       "10.129.0.29",
		"DstPort":       8443.0,
		"Packets_AB":    800.0,
		"Packets_BA":    1200.0,
		"Proto":         6.0,
		"SrcAddr":       "10.129.2.11",
		"SrcPort":       46894.0,
		"TimeFlowEnd":   1_637_501_829.0,
		"TimeFlowStart": 1_637_501_079.0,
		"_HashId":       "d28db42bcd8aea8f",
		"_RecordType":   api.ConnTrackOutputRecordTypeEnum("endConnection"),
		"_IsFirst":      false,
		"numFlowLogs":   5.0,
	}
	// Wait for the record to be eventually forwarded to the writer
	test2.Eventually(t, 15*time.Second, func(t require.TestingT) {
		require.Containsf(t, writer.AllRecords(), expected,
			"The output records don't include the expected record %v", expected)
	})
}

func ingestFile(t *testing.T, in chan<- config.GenericMap, filepath string) int {
	t.Helper()
	file, err := os.Open(filepath)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()
	var lines [][]byte
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		lines = append(lines, []byte(text))
	}
	submittedLines := 0
	decoder, err := decode.NewDecodeJSON()
	require.NoError(t, err)
	for _, line := range lines {
		line, err := decoder.Decode(line)
		require.NoError(t, err)
		in <- line
		submittedLines++
	}
	return submittedLines
}
