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
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const timeout = 5 * time.Second

type fakeEmitter struct {
	mock.Mock
	reorderJSON bool
}

func (f *fakeEmitter) Handle(labels model.LabelSet, timestamp time.Time, record string) error {
	if f.reorderJSON {
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
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)

	// Test that the API config was properly set and defaults applied
	assert.Equal(t, "https://foo:8888/", loki.apiConfig.URL)
	assert.Equal(t, "theTenant", loki.apiConfig.TenantID)
	assert.Equal(t, "1m", loki.apiConfig.BatchWait)
	assert.Equal(t, "5s", loki.apiConfig.MinBackoff)

	// Make sure default batch size is set
	assert.Equal(t, 102400, loki.apiConfig.BatchSize)
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

	// Test that the client was created successfully and API config is preserved
	assert.NotNil(t, loki.client)
	assert.NotNil(t, loki.apiConfig.ClientConfig)
}

func TestLoki_ProcessRecord(t *testing.T) {
	params := api.WriteLoki{
		URL:            "http://loki:3100/",
		TimestampLabel: "ts",
		IgnoreList:     []string{"ignored"},
		StaticLabels:   model.LabelSet{"static": "label"},
		Labels:         []string{"foo", "bar"},
	}
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(t, err)

	fe := fakeEmitter{reorderJSON: true}
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

func TestLoki_ProcessRecordOrdered(t *testing.T) {
	params := api.WriteLoki{
		URL:            "http://loki:3100/",
		TimestampLabel: "ts",
		Reorder:        true,
	}
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(t, err)

	fe := fakeEmitter{reorderJSON: false}
	fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loki.client = &fe

	require.NoError(t, loki.ProcessRecord(map[string]interface{}{"ts": 123456, "c": "c", "e": "e", "a": "a", "d": "d", "b": "b"}))

	fe.AssertCalled(t, "Handle", model.LabelSet{}, time.Unix(123456, 0), `{"a":"a","b":"b","c":"c","d":"d","e":"e","ts":123456}`)
}

func TestLoki_ProcessRecordGoFormat(t *testing.T) {
	params := api.WriteLoki{
		URL:            "http://loki:3100/",
		TimestampLabel: "ts",
		Labels:         []string{"foo"},
		Format:         "printf",
	}
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(t, err)

	fe := fakeEmitter{}
	fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loki.client = &fe

	// WHEN it processes input records
	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ts": 123456, "foo": "fooLabel", "bar": "barLabel", "value": 1234}))
	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ts": 124567, "foo": "fooLabel2", "bar": "barLabel2", "value": 5678, "other": "val"}))

	// THEN it forwards the records extracting the timestamp and labels from the configuration
	fe.AssertCalled(t, "Handle", model.LabelSet{"foo": "fooLabel"}, time.Unix(123456, 0), `map[bar:barLabel ts:123456 value:1234]`)
	fe.AssertCalled(t, "Handle", model.LabelSet{"foo": "fooLabel2"}, time.Unix(124567, 0), `map[bar:barLabel2 other:val ts:124567 value:5678]`)
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
	params := api.WriteLoki{
		URL:            "http://loki:3100/",
		TimestampLabel: "ts",
		Labels:         []string{"fo.o", "ba-r", "ba/z", "ignored?"},
	}
	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(t, err)

	fe := fakeEmitter{}
	fe.On("Handle", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loki.client = &fe

	require.NoError(t, loki.ProcessRecord(map[string]interface{}{
		"ba/z": "isBaz", "fo.o": "isFoo", "ba-r": "isBar", "ignored?": "yes!", "md": "md"}))

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
		"toIgnore":  "--",
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

func TestGRPCClientCreation(t *testing.T) {
	params := api.WriteLoki{
		URL:            "localhost:9095",
		ClientProtocol: "grpc",
		TenantID:       "test-tenant",
		GRPCConfig: &api.GRPCLokiConfig{
			KeepAlive:        "30s",
			KeepAliveTimeout: "5s",
		},
	}

	loki, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.NoError(t, err)
	require.NotNil(t, loki)
	require.NotNil(t, loki.client)
}

func TestGRPCClientCreationWithTLS(t *testing.T) {
	params := api.WriteLoki{
		URL:            "loki.example.com:9095",
		ClientProtocol: "grpc",
		TenantID:       "test-tenant",
		ClientConfig: &promConfig.HTTPClientConfig{
			TLSConfig: promConfig.TLSConfig{
				CertFile:           "/path/to/cert.pem",
				KeyFile:            "/path/to/key.pem",
				CAFile:             "/path/to/ca.pem",
				ServerName:         "loki.example.com",
				InsecureSkipVerify: false,
			},
		},
		GRPCConfig: &api.GRPCLokiConfig{
			KeepAlive:        "30s",
			KeepAliveTimeout: "5s",
		},
	}

	// This test expects to fail due to missing certificate files
	// We're testing the TLS config validation, not actual connection
	_, err := NewWriteLoki(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Write: &config.Write{Loki: &params}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}

func TestBuildGRPCLokiConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.WriteLoki
		wantErr  bool
		validate func(t *testing.T, cfg interface{})
	}{
		{
			name: "valid basic gRPC config",
			input: &api.WriteLoki{
				URL:            "localhost:9095",
				ClientProtocol: "grpc",
				BatchWait:      "2s",
				BatchSize:      50000,
				Timeout:        "15s",
				MinBackoff:     "2s",
				MaxBackoff:     "10s",
				MaxRetries:     5,
				TenantID:       "test-tenant",
				GRPCConfig: &api.GRPCLokiConfig{
					KeepAlive:        "60s",
					KeepAliveTimeout: "10s",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, cfg interface{}) {
				// Basic validation that config was created without error
				require.NotNil(t, cfg)
			},
		},
		{
			name: "missing gRPC config",
			input: &api.WriteLoki{
				URL:            "localhost:9095",
				ClientProtocol: "grpc",
				TenantID:       "test-tenant",
				GRPCConfig:     nil,
			},
			wantErr: true,
		},
		{
			name: "invalid duration in gRPC config",
			input: &api.WriteLoki{
				URL:            "localhost:9095",
				ClientProtocol: "grpc",
				BatchWait:      "invalid-duration",
				TenantID:       "test-tenant",
				GRPCConfig:     &api.GRPCLokiConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildGRPCLokiConfig(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestGRPCConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *api.GRPCLokiConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &api.GRPCLokiConfig{
				KeepAlive:        "30s",
				KeepAliveTimeout: "5s",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "invalid keepAlive duration",
			config: &api.GRPCLokiConfig{
				KeepAlive: "invalid-duration",
			},
			wantErr: true,
			errMsg:  "invalid keepAlive duration",
		},
		{
			name: "invalid keepAliveTimeout duration",
			config: &api.GRPCLokiConfig{
				KeepAlive:        "30s",
				KeepAliveTimeout: "invalid-timeout",
			},
			wantErr: true,
			errMsg:  "invalid keepAliveTimeout duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGRPCConfigDefaults(t *testing.T) {
	config := &api.GRPCLokiConfig{}

	config.SetDefaults()

	assert.Equal(t, "30s", config.KeepAlive)
	assert.Equal(t, "5s", config.KeepAliveTimeout)
}

func TestWriteLokiValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *api.WriteLoki
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid HTTP config",
			config: &api.WriteLoki{
				URL:            "http://localhost:3100",
				ClientProtocol: "http",
				BatchSize:      1024,
			},
			wantErr: false,
		},
		{
			name: "valid gRPC config",
			config: &api.WriteLoki{
				URL:            "localhost:9095",
				ClientProtocol: "grpc",
				BatchSize:      1024,
				GRPCConfig:     &api.GRPCLokiConfig{},
			},
			wantErr: false,
		},
		{
			name: "invalid client type",
			config: &api.WriteLoki{
				ClientProtocol: "websocket",
				BatchSize:      1024,
			},
			wantErr: true,
			errMsg:  "invalid clientProtocol",
		},
		{
			name: "missing URL for HTTP",
			config: &api.WriteLoki{
				ClientProtocol: "http",
				BatchSize:      1024,
			},
			wantErr: true,
			errMsg:  "url can't be empty",
		},
		{
			name: "missing URL for gRPC",
			config: &api.WriteLoki{
				ClientProtocol: "grpc",
				BatchSize:      1024,
				GRPCConfig:     &api.GRPCLokiConfig{},
			},
			wantErr: true,
			errMsg:  "url can't be empty",
		},
		{
			name: "missing gRPC config",
			config: &api.WriteLoki{
				URL:            "localhost:9095",
				ClientProtocol: "grpc",
				BatchSize:      1024,
			},
			wantErr: true,
			errMsg:  "grpcConfig is required",
		},
		{
			name: "invalid batch size",
			config: &api.WriteLoki{
				URL:            "http://localhost:3100",
				ClientProtocol: "http",
				BatchSize:      -1,
			},
			wantErr: true,
			errMsg:  "invalid batchSize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
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
		Labels:         []string{"srcIP", "dstIP", "toIgnore"},
		TimestampLabel: "timestamp",
		TimestampScale: "1ms",
		IgnoreList:     []string{"toIgnore"},
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
