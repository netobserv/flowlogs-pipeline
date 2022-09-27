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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testConfigTransformGenericMaintainFalse = `---
log-level: debug
pipeline:
  - name: transform1
parameters:
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
        - input: srcIP
          output: srcIP
`

const testConfigTransformGenericMaintainTrue = `---
log-level: debug
pipeline:
  - name: transform1
parameters:
  - name: transform1
    transform:
      type: generic
      generic:
        policy: preserve_original_keys
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

func getGenericExpectedOutputShort() config.GenericMap {
	return config.GenericMap{
		"SrcAddr":  "10.0.0.1",
		"srcIP":    "10.0.0.1",
		"SrcPort":  11777,
		"Protocol": "tcp",
		"DstAddr":  "20.0.0.2",
		"DstPort":  22,
	}
}

func getGenericExpectedOutputLong() config.GenericMap {
	return config.GenericMap{
		"SrcAddr":      "10.0.0.1",
		"SrcPort":      11777,
		"Protocol":     "tcp",
		"DstAddr":      "20.0.0.2",
		"DstPort":      22,
		"srcIP":        "10.0.0.1",
		"8888IP":       "8.8.8.8",
		"emptyIP":      "",
		"level":        "error",
		"srcPort":      11777,
		"protocol":     "tcp",
		"protocol_num": 6,
		"value":        7.0,
		"message":      "test message",
		"dstIP":        "20.0.0.2",
		"dstPort":      22,
	}
}

func TestNewTransformGenericMaintainFalse(t *testing.T) {
	newTransform := InitNewTransformGeneric(t, testConfigTransformGenericMaintainFalse)
	transformGeneric := newTransform.(*Generic)
	require.Len(t, transformGeneric.rules, 6)

	input := test.GetIngestMockEntry(false)
	output, ok := transformGeneric.Transform(input)
	require.True(t, ok)
	expectedOutput := getGenericExpectedOutputShort()
	require.Equal(t, expectedOutput, output)
}

func TestNewTransformGenericMaintainTrue(t *testing.T) {
	newTransform := InitNewTransformGeneric(t, testConfigTransformGenericMaintainTrue)
	transformGeneric := newTransform.(*Generic)
	require.Len(t, transformGeneric.rules, 6)

	input := test.GetIngestMockEntry(false)
	output, ok := transformGeneric.Transform(input)
	require.True(t, ok)
	expectedOutput := getGenericExpectedOutputLong()
	require.Equal(t, expectedOutput, output)
}

func InitNewTransformGeneric(t *testing.T, configFile string) Transformer {
	v, cfg := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	config := cfg.Parameters[0]
	newTransform, err := NewTransformGeneric(config)
	require.NoError(t, err)
	return newTransform
}
