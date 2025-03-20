/*
 * Copyright (C) 2023 IBM, Inc.
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

package opentelemetry

import (
	"encoding/json"
	"testing"

	otel "github.com/agoda-com/opentelemetry-logs-go"
	"github.com/agoda-com/opentelemetry-logs-go/logs"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testOtlpConfig = `---
log-level: debug
pipeline:
  - name: encode1
parameters:
  - name: encode1
    encode:
      type: otlplogs
      otlplogs:
        address: localhost
        port: 4317
        connectionType: grpc
`

type fakeOltpLoggerProvider struct {
}
type fakeOltpLogger struct {
}

var otlpReceivedData []logs.LogRecord

// nolint:gocritic
func (f *fakeOltpLogger) Emit(msg logs.LogRecord) {
	otlpReceivedData = append(otlpReceivedData, msg)
}

func (f *fakeOltpLoggerProvider) Logger(_ string, _ ...logs.LoggerOption) logs.Logger {
	return &fakeOltpLogger{}
}

func initNewEncodeOtlpLogs(t *testing.T) encode.Encoder {
	otlpReceivedData = []logs.LogRecord{}
	v, cfg := test.InitConfig(t, testOtlpConfig)
	require.NotNil(t, v)

	newEncode, err := NewEncodeOtlpLogs(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)

	// intercept the loggerProvider function
	loggerProvider := fakeOltpLoggerProvider{}
	otel.SetLoggerProvider(&loggerProvider)
	return newEncode
}

func Test_EncodeOtlpLogs(t *testing.T) {
	newEncode := initNewEncodeOtlpLogs(t)
	encodeOtlp := newEncode.(*EncodeOtlpLogs)
	require.Equal(t, "localhost", encodeOtlp.cfg.OtlpConnectionInfo.Address)
	require.Equal(t, 4317, encodeOtlp.cfg.OtlpConnectionInfo.Port)
	require.Equal(t, "grpc", encodeOtlp.cfg.ConnectionType)
	require.Nil(t, encodeOtlp.cfg.TLS)
	require.Empty(t, encodeOtlp.cfg.Headers)

	entry1 := test.GetIngestMockEntry(true)
	entry2 := test.GetIngestMockEntry(false)
	newEncode.Encode(entry1)
	newEncode.Encode(entry2)
	// verify contents of the output
	require.Len(t, otlpReceivedData, 2)
	expected1, _ := json.Marshal(entry1)
	expected2, _ := json.Marshal(entry2)
	body1 := otlpReceivedData[0].Body()
	body2 := otlpReceivedData[1].Body()
	require.Equal(t, string(expected1), body1.(string))
	require.Equal(t, string(expected2), body2.(string))
}

func Test_EncodeOtlpTLSConfigEmpty(t *testing.T) {
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpLogs: &api.EncodeOtlpLogs{OtlpConnectionInfo: &api.OtlpConnectionInfo{
				Address:        "1.2.3.4",
				Port:           999,
				TLS:            nil,
				ConnectionType: "grpc",
				Headers:        nil,
			}}},
	}
	newEncode, err := NewEncodeOtlpLogs(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.NoError(t, err)
	require.NotNil(t, newEncode)
}

func Test_EncodeOtlpTLSConfigNotEmpty(t *testing.T) {
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpLogs: &api.EncodeOtlpLogs{OtlpConnectionInfo: &api.OtlpConnectionInfo{
				Address:        "1.2.3.4",
				Port:           999,
				ConnectionType: "grpc",
				TLS:            &api.ClientTLS{InsecureSkipVerify: true},
			}}},
	}
	newEncode, err := NewEncodeOtlpLogs(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.NoError(t, err)
	require.NotNil(t, newEncode)
}

func Test_HeadersNotEmpty(t *testing.T) {
	headers := make(map[string]string)
	headers["key1"] = "value1"
	headers["key2"] = "value2"
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpLogs: &api.EncodeOtlpLogs{OtlpConnectionInfo: &api.OtlpConnectionInfo{
				Address:        "1.2.3.4",
				Port:           999,
				ConnectionType: "grpc",
				Headers:        headers,
			}}},
	}
	newEncode, err := NewEncodeOtlpLogs(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.NoError(t, err)
	require.NotNil(t, newEncode)
}

func Test_NoConnectionType(t *testing.T) {
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpLogs: &api.EncodeOtlpLogs{OtlpConnectionInfo: &api.OtlpConnectionInfo{
				Address: "1.2.3.4",
				Port:    999,
			}}},
	}
	newEncode, err := NewEncodeOtlpLogs(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.Error(t, err)
	require.Nil(t, newEncode)
}

func Test_EncodeOtlpTraces(t *testing.T) {
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpTraces: &api.EncodeOtlpTraces{OtlpConnectionInfo: &api.OtlpConnectionInfo{
				Address:        "1.2.3.4",
				Port:           999,
				ConnectionType: "grpc",
				Headers:        nil,
			}}},
	}
	newEncode, err := NewEncodeOtlpTraces(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.NoError(t, err)
	require.NotNil(t, newEncode)
}

func Test_EncodeOtlpMetrics(t *testing.T) {
	cfg := config.StageParam{
		Encode: &config.Encode{
			OtlpMetrics: &api.EncodeOtlpMetrics{
				OtlpConnectionInfo: &api.OtlpConnectionInfo{
					Address:        "1.2.3.4",
					Port:           999,
					ConnectionType: "grpc",
					Headers:        nil,
				},
				Prefix: "flp_test",
				Metrics: []api.MetricsItem{
					{Name: "metric1", Type: "counter", Labels: []string{"label11", "label12"}},
					{Name: "metric2", Type: "gauge", Labels: []string{"label21", "label22"}},
					{Name: "metric3", Type: "counter", Labels: []string{"label31", "label32"}},
				},
			}},
	}
	newEncode, err := NewEncodeOtlpMetrics(operational.NewMetrics(&config.MetricsSettings{}), cfg)
	require.NoError(t, err)
	require.NotNil(t, newEncode)
	// TODO: add more tests
}
