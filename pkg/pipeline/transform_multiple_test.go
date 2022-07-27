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

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testConfigTransformMultiple = `---
log-level: debug
pipeline:
  - name: ingest1
  - name: transform1
    follows: ingest1
  - name: transform2
    follows: transform1
  - name: transform3
    follows: transform2
  - name: writer1
    follows: transform3
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
        - input: srcIP
          output: SrcAddr
        - input: dstIP
          output: DstAddr
        - input: dstPort
          output: DstPort
        - input: srcPort
          output: SrcPort
        - input: protocol
          output: Protocol
  - name: transform2
    transform:
      type: none
  - name: transform3
    transform:
      type: generic
      generic:
        policy: replace_keys
        rules:
        - input: SrcAddr
          output: SrcAddr2
        - input: DstAddr
          output: DstAddr2
        - input: DstPort
          output: DstPort2
        - input: SrcPort
          output: SrcPort2
        - input: Protocol
          output: Protocol2
  - name: writer1
    write:
      type: none
`

func TestTransformMultiple(t *testing.T) {
	var mainPipeline *Pipeline
	var err error
	v, cfg := test.InitConfig(t, testConfigTransformMultiple)
	require.NotNil(t, v)

	mainPipeline, err = NewPipeline(cfg)
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()
	// TODO: We should test the final outcome to see that it is reasonable
}
