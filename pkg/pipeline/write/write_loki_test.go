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

package write

import (
	"encoding/json"
	"fmt"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeEmitter struct {
	mock.Mock
}

func (f *fakeEmitter) Handle(labels model.LabelSet, timestamp time.Time, record string) error {
	a := f.Mock.Called(labels, timestamp, record)
	return a.Error(0)
}

func Test_buildLokiConfig(t *testing.T) {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        tenantID: theTenant
        url: "https://foo:8888/"
        batchWait: 1m
        minBackOff: 5s
        labels:
          - foo
          - bar
        staticLabels:
          baz: bae
          tiki: taka
`
	v := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(config.Parameters[0])
	require.NoError(t, err)

	assert.Equal(t, "https://foo:8888/loki/api/v1/push", loki.lokiConfig.URL.String())
	assert.Equal(t, "theTenant", loki.lokiConfig.TenantID)
	assert.Equal(t, time.Minute, loki.lokiConfig.BatchWait)
	assert.NotZero(t, loki.lokiConfig.BatchSize)
	assert.Equal(t, loki.apiConfig.BatchSize, loki.lokiConfig.BatchSize)
	minBackoff, _ := time.ParseDuration(loki.apiConfig.MinBackoff)
	assert.Equal(t, minBackoff, loki.lokiConfig.BackoffConfig.MinBackoff)
}

func TestLoki_ProcessRecord(t *testing.T) {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        timestampLabel: ts
        ignoreList:
        - ignored
        staticLabels:
          static: label
        labels:
          - foo
          - bar
`
	v := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(config.Parameters[0])
	require.NoError(t, err)

	fe := fakeEmitter{}
	fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loki.client = &fe

	// WHEN it processes input records
	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ts": 123456, "ignored": "ignored!", "foo": "fooLabel", "bar": "barLabel", "value": 1234}))
	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ts": 124567, "ignored": "ignored!", "foo": "fooLabel2", "bar": "barLabel2", "value": 5678, "other": "val"}))

	// THEN it forwards the records extracting the timestamp and labels from the configuration
	fe.AssertCalled(t, "Handle", model.LabelSet{
		"bar":    "barLabel",
		"foo":    "fooLabel",
		"static": "label",
	}, time.Unix(123456, 0), `{"ts":123456,"value":1234}`)

	fe.AssertCalled(t, "Handle", model.LabelSet{
		"bar":    "barLabel2",
		"foo":    "fooLabel2",
		"static": "label",
	}, time.Unix(124567, 0), `{"other":"val","ts":124567,"value":5678}`)
}

func TestTimestampScale(t *testing.T) {
	// verifies that the unix residual time (below 1-second precision) is properly
	// incorporated into the timestamp whichever scale it is
	for _, testCase := range []struct {
		unit     string
		expected time.Time
	}{
		{unit: "1m", expected: time.Unix(123456789*60, 0)},
		{unit: "1s", expected: time.Unix(123456789, 0)},
		{unit: "100ms", expected: time.Unix(12345678, 900000000)},
		{unit: "1ms", expected: time.Unix(123456, 789000000)},
	} {
		t.Run(fmt.Sprintf("unit %v", testCase.unit), func(t *testing.T) {
			yamlConf := fmt.Sprintf(`log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        timestampScale: %s
`, testCase.unit)
			v := test.InitConfig(t, yamlConf)
			require.NotNil(t, v)

			loki, err := NewWriteLoki(config.Parameters[0])
			require.NoError(t, err)

			fe := fakeEmitter{}
			fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			loki.client = &fe

			require.NoError(t, loki.ProcessRecord(map[string]interface{}{"TimeReceived": 123456789}))
			fe.AssertCalled(t, "Handle", model.LabelSet{},
				testCase.expected, `{"TimeReceived":123456789}`)
		})
	}
}

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

// Tests those cases where the timestamp can't be extracted and reports the current time
func TestTimestampExtraction_LocalTime(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		tsLabel model.LabelName
		input   map[string]interface{}
	}{
		{name: "undefined ts label", tsLabel: "", input: map[string]interface{}{"ts": 444}},
		{name: "non-existing ts entry", tsLabel: "asdfasdf", input: map[string]interface{}{"ts": 444}},
		{name: "non-numeric ts value", tsLabel: "ts", input: map[string]interface{}{"ts": "string value"}},
		{name: "zero ts value", tsLabel: "ts", input: map[string]interface{}{"ts": 0}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			v := test.InitConfig(t, yamlConfigNoParams)
			require.NotNil(t, v)

			loki, err := NewWriteLoki(config.Parameters[0])
			require.NoError(t, err)

			loki.apiConfig.TimestampLabel = testCase.tsLabel

			fe := fakeEmitter{}
			fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			loki.client = &fe

			loki.timeNow = func() time.Time {
				return time.Unix(12345678, 0)
			}
			jsonInput, _ := json.Marshal(testCase.input)
			require.NoError(t, loki.ProcessRecord(testCase.input))
			fe.AssertCalled(t, "Handle", model.LabelSet{},
				time.Unix(12345678, 0), string(jsonInput))
		})
	}
}

// Tests that labels are sanitized before being sent to loki.
// Labels that are invalid even if sanitized are ignored
func TestSanitizedLabels(t *testing.T) {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        labels:
          - "fo.o"
          - "ba-r"
          - "ba/z"
          - "ignored?"
`
	v := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(config.Parameters[0])
	require.NoError(t, err)

	fe := fakeEmitter{}
	fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loki.client = &fe

	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ba/z": "isBaz", "fo.o": "isFoo", "ba-r": "isBar", "ignored?": "yes!"}))

	fe.AssertCalled(t, "Handle", model.LabelSet{
		"ba_r": "isBar",
		"fo_o": "isFoo",
		"ba_z": "isBaz",
	}, mock.Anything, mock.Anything)
}
