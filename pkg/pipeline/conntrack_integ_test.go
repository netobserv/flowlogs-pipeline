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
	"testing"
	"time"

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
        endConnectionTimeout: 1s
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
	v := test.InitConfig(t, testConfigConntrack)
	require.NotNil(t, v)

	mainPipeline, err = NewPipeline()
	require.NoError(t, err)

	go mainPipeline.Run()

	in := mainPipeline.pipelineStages[0].Ingester.(*ingest.IngestFake).In
	writer := mainPipeline.pipelineStages[2].Writer.(*write.WriteFake)

	ingestFile(t, in, "../../hack/examples/ocp-ipfix-flowlogs.json")
	writer.Wait()
	writer.ResetWait()

	// Wait a moment to make the connections expired
	time.Sleep(2 * time.Second)
	// Send an empty list to the pipeline to allow the connection tracking output end connection records
	in <- []config.GenericMap{}
	writer.Wait()

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
		"_RecordType":   "endConnection",
		"numFlowLogs":   5.0,
	}
	require.Containsf(t, writer.AllRecords, expected, "The output records don't include the expected record %v", expected)
}

func ingestFile(t *testing.T, in chan<- []config.GenericMap, filepath string) {
	t.Helper()
	file, err := os.Open(filepath)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()
	lines := make([]interface{}, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		lines = append(lines, text)
	}
	decoder, err := decode.NewDecodeJson()
	require.NoError(t, err)
	decoded := decoder.Decode(lines)
	in <- decoded
}
