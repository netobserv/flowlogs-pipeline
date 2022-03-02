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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
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
	input := config.GenericMap{"key": "value"}
	transform, err := transform.NewTransformNone()
	require.NoError(t, err)
	transformed = append(transformed, transform.Transform(input))

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
	require.Equal(t, len(ingester.PrevRecords), len(decoder.PrevRecords))
}
