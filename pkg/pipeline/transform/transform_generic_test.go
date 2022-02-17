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

const testConfigTransformGeneric = `---
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
        - input: srcIP
          output: srcIP
`

func getGenericExpectedOutput() config.GenericMap {
	return config.GenericMap{
		"SrcAddr":  "10.0.0.1",
		"srcIP":    "10.0.0.1",
		"SrcPort":  11777,
		"Protocol": "tcp",
		"DstAddr":  "20.0.0.2",
		"DstPort":  22,
	}
}

func TestNewTransformGeneric(t *testing.T) {
	newTransform := InitNewTransform(t, testConfigTransformGeneric)
	transformGeneric := newTransform.(*Generic)
	require.Equal(t, len(transformGeneric.Rules), 6)

	input := test.GetIngestMockEntry(false)
	output := transformGeneric.Transform(input)
	expectedOutput := getGenericExpectedOutput()
	require.Equal(t, output, expectedOutput)
}

func InitNewTransform(t *testing.T, configFile string) Transformer {
	v := test.InitConfig(t, configFile)
	val := v.Get("pipeline.transform")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)

	// perform initializations usually done in main.go
	config.Opt.PipeLine.Transform = string(b)

	newTransforms, err := GetTransformers()
	require.Equal(t, err, nil)
	return newTransforms[0]
}
