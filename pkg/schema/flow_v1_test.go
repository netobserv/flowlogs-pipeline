/*
 * Copyright (C) 2026 NetObserv Authors.
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

package schema

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowTimestampMs(t *testing.T) {
	ts, ok := FlowTimestampMs(config.GenericMap{"TimeFlowEndMs": int64(2000), "TimeFlowStartMs": int64(1000)})
	require.True(t, ok)
	assert.Equal(t, int64(2000), ts)

	ts, ok = FlowTimestampMs(config.GenericMap{"TimeReceived": int64(1700000000)})
	require.True(t, ok)
	assert.Equal(t, int64(1700000000000), ts)
}

func TestFromGenericMap(t *testing.T) {
	rec := FromGenericMap(config.GenericMap{
		"TimeFlowStartMs": int64(1),
		"TimeFlowEndMs":   int64(2),
		"SrcAddr":         "10.0.0.1",
		"SrcPort":         80,
		"Bytes":           int64(100),
		"SrcK8S_Name":     "pod-a",
		"TimeFlowRttNs":   int64(12345),
	})
	assert.Equal(t, int64(1), rec.TimeFlowStartMs)
	assert.Equal(t, "10.0.0.1", rec.SrcAddr)
	require.NotNil(t, rec.SrcPort)
	assert.Equal(t, int32(80), *rec.SrcPort)
	require.NotNil(t, rec.Bytes)
	assert.Equal(t, int64(100), *rec.Bytes)
	assert.Equal(t, "pod-a", rec.SrcK8S_Name)
	require.NotNil(t, rec.TimeFlowRttNs)
	assert.Equal(t, int64(12345), *rec.TimeFlowRttNs)
	assert.Nil(t, rec.DnsLatencyMs)
}
