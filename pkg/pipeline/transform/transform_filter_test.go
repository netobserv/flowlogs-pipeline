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

const testConfigTransformFilterRemoveField = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
          - input: dstPort
            type: remove_field
          - input: srcPort
            type: remove_field
`

const testConfigTransformFilterRemoveEntryIfExists = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
          - input: srcPort
            type: remove_entry_if_exists
`

const testConfigTransformFilterRemoveEntryIfDoesntExists = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
          - input: doesntSrcPort
            type: remove_entry_if_doesnt_exist
`

func getFilterExpectedOutput() config.GenericMap {
	return config.GenericMap{
		"srcIP":        "10.0.0.1",
		"8888IP":       "8.8.8.8",
		"emptyIP":      "",
		"level":        "error",
		"protocol":     "tcp",
		"protocol_num": 6,
		"value":        7.0,
		"message":      "test message",
		"dstIP":        "20.0.0.2",
	}
}

func TestNewTransformFilterRemoveField(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveField)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)
	output := transformFilter.Transform([]config.GenericMap{input})
	expectedOutput := getFilterExpectedOutput()
	require.Equal(t, expectedOutput, output[0])
}

func TestNewTransformFilterRemoveEntryIfExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	output := transformFilter.Transform([]config.GenericMap{input})
	require.Equal(t, output, []config.GenericMap{})
}

func TestNewTransformFilterRemoveEntryIfDoesntExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfDoesntExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	output := transformFilter.Transform([]config.GenericMap{input})
	require.Equal(t, output, []config.GenericMap{})
}
func InitNewTransformFilter(t *testing.T, configFile string) Transformer {
	v := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	config := config.Parameters[0]
	newTransform, err := NewTransformFilter(config)
	require.NoError(t, err)
	return newTransform
}
