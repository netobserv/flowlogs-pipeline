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

package transform

import (
	"github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
)

const testConfigTransformMultiple = `---
log-level: debug
pipeline:
  transform:
    - type: generic
      generic:
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
    - type: none
    - type: generic
      generic:
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
`

func getMultipleExpectedOutput() config.GenericMap {
	return config.GenericMap{
		"SrcAddr2":  "10.0.0.1",
		"SrcPort2":  "11777",
		"Protocol2": "tcp",
		"DstAddr2":  "20.0.0.2",
		"DstPort2":  "22",
	}
}

func InitMultipleTransforms(t *testing.T, configFile string) []Transformer {
	v := test.InitConfig(t, configFile)
	val := v.Get("pipeline.transform")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)

	// perform initializations usually done in main.go
	config.Opt.PipeLine.Transform = string(b)

	newTransforms, err := GetTransformers()
	require.Equal(t, err, nil)
	return newTransforms
}

func TestNewTransformMultiple(t *testing.T) {
	newTransforms := InitMultipleTransforms(t, testConfigTransformMultiple)
	require.Equal(t, len(newTransforms), 3)

	input := test.GetIngestMockEntry(false)
	output := ExecuteTransforms(newTransforms, input)
	expectedOutput := getMultipleExpectedOutput()
	require.Equal(t, output, expectedOutput)
}
