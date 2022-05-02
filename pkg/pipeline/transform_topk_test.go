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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testConfigAggregateTopK = `---
log-level: debug
pipeline:
  - name: ingest_file
  - follows: ingest_file
    name: decode_json
  - follows: decode_json
    name: transform_generic
  - follows: transform_generic
    name: transform_network
  - follows: transform_network
    name: extract_aggregate
  - follows: extract_aggregate
    name: write_none
parameters:
  - ingest:
      type: file
      file:
        filename: ../../hack/examples/ocp-ipfix-flowlogs.json
    name: ingest_file
  - decode:
      type: json
    name: decode_json
  - name: transform_generic
    transform:
      generic:
        policy: replace_keys
        rules:
        - input: SrcAddr
          output: srcIP
        - input: SrcPort
          output: srcPort
        - input: DstAddr
          output: dstIP
        - input: DstPort
          output: dstPort
        - input: Proto
          output: proto
        - input: Bytes
          output: bytes
        - input: TCPFlags
          output: TCPFlags
        - input: SrcAS
          output: srcAS
        - input: DstAS
          output: dstAS
      type: generic
  - name: transform_network
    transform:
      network:
        rules:
        - input: dstIP
          output: dstSubnet24
          type: add_subnet
          parameters: /24
        - input: srcIP
          output: srcSubnet24
          type: add_subnet
          parameters: /24
      type: network
  - extract:
      aggregates:
      - Name: count_source_destination_subnet
        By:
        - dstSubnet24
        - srcSubnet24
        Operation: count
        RecordKey: ""
        TopK: 4
      type: aggregates
    name: extract_aggregate
  - name: write_none
    write:
      type: none
`

func TestAggregateTopk(t *testing.T) {
	var mainPipeline *Pipeline
	var err error
	v := test.InitConfig(t, testConfigAggregateTopK)
	require.NotNil(t, v)

	mainPipeline, err = NewPipeline()
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()
	// Test the final outcome to see that it is reasonable
	extractor := mainPipeline.pipelineStages[4].Extractor.(*extract.ExtractAggregate)
	writer := mainPipeline.pipelineStages[5].Writer.(*write.WriteNone)
	require.Equal(t, 4, extractor.Aggregates.Aggregates[0].Definition.TopK)
	require.Equal(t, 4, len(writer.PrevRecords))
}
