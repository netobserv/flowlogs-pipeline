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
	"math/rand"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const timeout = 5 * time.Second

type fakeEmitter struct {
	mock.Mock
}

func (f *fakeEmitter) Handle(labels model.LabelSet, timestamp time.Time, record string) error {
	// sort alphabetically records just for simplifying testing verification with JSON strings
	recordMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(record), &recordMap); err != nil {
		panic("expected JSON: " + err.Error())
	}
	recordBytes, err := json.Marshal(recordMap)
	if err != nil {
		panic("error unmarshaling: " + err.Error())
	}
	a := f.Mock.Called(labels, timestamp, string(recordBytes))
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
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)

	assert.Equal(t, "https://foo:8888/loki/api/v1/push", loki.lokiConfig.URL.String())
	assert.Equal(t, "theTenant", loki.lokiConfig.TenantID)
	assert.Equal(t, time.Minute, loki.lokiConfig.BatchWait)
	minBackoff, _ := time.ParseDuration(loki.apiConfig.MinBackoff)
	assert.Equal(t, minBackoff, loki.lokiConfig.BackoffConfig.MinBackoff)

	// Make sure default batch size is set
	assert.Equal(t, 102400, loki.lokiConfig.BatchSize)
	assert.Equal(t, loki.apiConfig.BatchSize, loki.lokiConfig.BatchSize)
}

func Test_buildLokiConfig_ClientDeserialization(t *testing.T) {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        url: "https://foo:8888/"
        clientConfig: {"tls_config":{"insecure_skip_verify":true},"follow_redirects":false,"enable_http2":false,"proxy_url":null}
`
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	// Due to an issue in prometheus/common HTTP client serde, a null proxy_url ends up as ""
	// see also https://github.com/netobserv/prometheus-common/commit/df8298253f033b910644548b6902df9d34baa752 (won't be merged upstream)
	// However now this shouldn't have any bad effect anymore, as the proxyFunc is now nil
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)

	proxyFunc := loki.lokiConfig.Client.Proxy()
	assert.Nil(t, proxyFunc)
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
        url: http://loki:3100/
        timestampLabel: ts
        ignoreList:
        - ignored
        staticLabels:
          static: label
        labels:
          - foo
          - bar
`
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
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
        url: http://loki:3100/
        timestampScale: %s
`, testCase.unit)
			v, cfg := test.InitConfig(t, yamlConf)
			require.NotNil(t, v)

			loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
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
      loki: { url: http://loki:3100/ }
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
			v, cfg := test.InitConfig(t, yamlConfigNoParams)
			require.NotNil(t, v)

			loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
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
        url: http://loki:3100/
        labels:
          - "fo.o"
          - "ba-r"
          - "ba/z"
          - "ignored?"
`
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
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

func TestHTTPInvocations(t *testing.T) {
	lokiFlows := make(chan map[string]interface{}, 256)
	fakeLoki := httptest.NewServer(test.FakeLokiHandler(lokiFlows))

	var yamlConfig = fmt.Sprintf(`
log-level: debug
pipeline:
  - name: write1
parameters:
  - name: write1
    write:
      type: loki
      loki:
        url: %s
`, fakeLoki.URL)
	_, cfg := test.InitConfig(t, yamlConfig)
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)

	require.NoError(t, loki.ProcessRecord(map[string]interface{}{"foo": "bar", "baz": "bae"}))

	select {
	case flow := <-lokiFlows:
		assert.Equal(t, map[string]interface{}{"foo": "bar", "baz": "bae"}, flow)
	case <-time.After(timeout):
		require.Fail(t, "timeout while waiting for LokiWriter to forward data")
	}
}

func buildFlow(t time.Time) config.GenericMap {
	return config.GenericMap{
		"timestamp": float64(t.UnixMilli()),
		"srcIP":     "10.0.0." + strconv.Itoa(rand.Intn(20)),
		"dstIP":     "10.0.0." + strconv.Itoa(rand.Intn(20)),
		"flags":     "SYN",
		"bytes":     rand.Intn(100),
		"packets":   rand.Intn(10),
		"latency":   rand.Float64(),
	}
}

func hundredFlows() []config.GenericMap {
	flows := make([]config.GenericMap, 100)
	t := time.Date(2022, time.August, 31, 8, 0, 0, 0, time.Local)
	for i := 0; i < 100; i++ {
		t = t.Add(time.Second)
		flows[i] = buildFlow(t)
	}
	return flows
}

func BenchmarkWriteLoki(b *testing.B) {
	logrus.SetLevel(logrus.ErrorLevel)
	lokiFlows := make(chan map[string]interface{}, 256)
	fakeLoki := httptest.NewServer(test.FakeLokiHandler(lokiFlows))

	params := api.WriteLoki{
		URL: fakeLoki.URL,
		StaticLabels: model.LabelSet{
			"app": "flp-benchmark",
		},
		Labels:         []string{"srcIP", "dstIP"},
		TimestampLabel: "timestamp",
		TimestampScale: "1ms",
	}

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(b, err)

	hf := hundredFlows()
	for i := 0; i < b.N; i++ {
		for _, f := range hf {
			loki.Write(f)
		}
	}
}
