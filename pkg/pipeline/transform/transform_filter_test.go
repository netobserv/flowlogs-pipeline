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
const testConfigTransformFilterRemoveEntryIfEqual = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - input: message
          type: remove_entry_if_equal
          value: "test message"
        - input: value
          type: remove_entry_if_equal
          value: 8.0
`

const testConfigTransformFilterRemoveEntryIfNotEqual = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - input: message
          type: remove_entry_if_not_equal
          value: "test message"
`

const testConfigTransformFilterAddField = `---
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
          type: add_field_if_doesnt_exist
          value: dummy_value
        - input: dummy_field
          type: add_field_if_doesnt_exist
          value: dummy_value
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
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	expectedOutput := getFilterExpectedOutput()
	require.Equal(t, expectedOutput, output)
}

func TestNewTransformFilterRemoveEntryIfExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	_, ok := transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfDoesntExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfDoesntExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	_, ok := transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfEqual(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfEqual)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)

	_, ok := transformFilter.Transform(input)
	require.False(t, ok)

	input["message"] = "dummy message"
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, output["message"], "dummy message")

	input["value"] = 8.0
	_, ok = transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfNotEqual(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfNotEqual)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)

	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, output["message"], "test message")

	input["message"] = "dummy message"
	_, ok = transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterAddField(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterAddField)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, 22, output["dstPort"])
	require.Equal(t, "dummy_value", output["dummy_field"])

	input = test.GetIngestMockEntry(false)
	input["dstPort"] = 3490
	input["dummy_field"] = 1
	output, ok = transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, 3490, output["dstPort"])
	require.Equal(t, 1, output["dummy_field"])
}

func InitNewTransformFilter(t *testing.T, configFile string) Transformer {
	v, cfg := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	config := cfg.Parameters[0]
	newTransform, err := NewTransformFilter(config)
	require.NoError(t, err)
	return newTransform
}
