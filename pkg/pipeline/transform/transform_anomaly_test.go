/*
 * Copyright (C) 2024 IBM, Inc.
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

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfigTransformAnomaly = `---
log-level: debug
pipeline:
  - name: anomaly1
parameters:
  - name: anomaly1
    transform:
      type: anomaly
      anomaly:
        algorithm: zscore
        valueField: bytes
        keyFields: [srcIP, dstIP]
        windowSize: 5
        baselineWindow: 3
        sensitivity: 2.5
`

func initNewTransformAnomaly(t *testing.T, configFile string) *Anomaly {
	v, cfg := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	configParams := cfg.Parameters[0]
	newTransform, err := NewTransformAnomaly(configParams, operational.NewMetrics(nil))
	require.NoError(t, err)
	return newTransform.(*Anomaly)
}

func TestNewTransformAnomalyConfig(t *testing.T) {
	anomaly := initNewTransformAnomaly(t, testConfigTransformAnomaly)
	require.Equal(t, 5, anomaly.windowSize)
	require.Equal(t, 3, anomaly.baselineWindow)
	require.Equal(t, 2.5, anomaly.sensitivity)
	require.Equal(t, api.AnomalyAlgorithmZScore, anomaly.config.Algorithm)
}

func TestTransformAnomalyZScore(t *testing.T) {
	anomaly := initNewTransformAnomaly(t, testConfigTransformAnomaly)
	baseFlow := func(val float64) config.GenericMap {
		return config.GenericMap{"srcIP": "1.1.1.1", "dstIP": "2.2.2.2", "bytes": val}
	}
	// warmup period
	for _, v := range []float64{100, 102, 98} {
		out, ok := anomaly.Transform(baseFlow(v))
		require.True(t, ok)
		assert.Equal(t, "warming_up", out["anomaly_type"])
	}

	normalOut, ok := anomaly.Transform(baseFlow(101))
	require.True(t, ok)
	assert.Equal(t, "normal", normalOut["anomaly_type"])

	anomalousOut, ok := anomaly.Transform(baseFlow(250))
	require.True(t, ok)
	assert.NotEqual(t, "normal", anomalousOut["anomaly_type"])
	score := anomalousOut["anomaly_score"].(float64)
	assert.GreaterOrEqual(t, score, anomaly.sensitivity)
}

func TestTransformAnomalyReset(t *testing.T) {
	anomaly := initNewTransformAnomaly(t, testConfigTransformAnomaly)
	flow := config.GenericMap{"srcIP": "10.0.0.1", "dstIP": "2.2.2.2", "bytes": 50}
	for i := 0; i < 3; i++ {
		_, _ = anomaly.Transform(flow)
	}
	anomaly.Reset()
	out, ok := anomaly.Transform(flow)
	require.True(t, ok)
	assert.Equal(t, "warming_up", out["anomaly_type"])
}

func BenchmarkTransformAnomaly(b *testing.B) {
	anomaly := initNewTransformAnomaly(&testing.T{}, testConfigTransformAnomaly)
	flow := config.GenericMap{"srcIP": "1.1.1.1", "dstIP": "2.2.2.2", "bytes": 100.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = anomaly.Transform(flow)
	}
}
