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

package config

import (
	"encoding/json"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestJSONUnmarshalStrict(t *testing.T) {
	type Message struct {
		Foo int    `json:"F"`
		Bar string `json:"B"`
	}
	msg := `{"F":1, "B":"bbb"}`
	var actualMsg Message
	expectedMsg := Message{Foo: 1, Bar: "bbb"}
	err := JSONUnmarshalStrict([]byte(msg), &actualMsg)
	require.NoError(t, err)
	require.Equal(t, expectedMsg, actualMsg)

	msg = `{"F":1, "B":"bbb", "NewField":0}`
	err = JSONUnmarshalStrict([]byte(msg), &actualMsg)
	require.Error(t, err)
}

func TestUnmarshalInline(t *testing.T) {
	cfg := `{"metricsSettings":{"port":9102,"prefix":"netobserv_"}}`
	var cfs Root
	err := yaml.Unmarshal([]byte(cfg), &cfs)
	require.NoError(t, err)
	require.Equal(t, "netobserv_", cfs.MetricsSettings.Prefix)
	require.Equal(t, 9102, cfs.MetricsSettings.PromConnectionInfo.Port)

	err = json.Unmarshal([]byte(cfg), &cfs)
	require.NoError(t, err)
	require.Equal(t, "netobserv_", cfs.MetricsSettings.Prefix)
	require.Equal(t, 9102, cfs.MetricsSettings.PromConnectionInfo.Port)
}

func TestUnmarshalConfig(t *testing.T) {
	input := `{"parameters":[{"name":"loki","write":{"type":"loki","loki":{"tenantID":"netobserv","clientConfig":{"proxy_url":null}}}}]}`
	sections, err := FileSections([]byte(input))
	require.NoError(t, err)
	cfs, err := ParseConfig(&Options{
		PipeLine:   "[]",
		Parameters: string(sections["parameters"]),
	})
	require.NoError(t, err)
	assert.Equal(t, "netobserv", cfs.Parameters[0].Write.Loki.TenantID)
}

func TestFileSectionsPreservesSectionKeyCase(t *testing.T) {
	sections, err := FileSections([]byte("Pipeline: []\n"))
	require.NoError(t, err)
	assert.Contains(t, sections, "Pipeline")
	assert.NotContains(t, sections, "pipeline")
}

func TestConnTrackSelectorConfigPreservesFieldNameCase(t *testing.T) {
	input := `{"parameters":[{"name":"conntrack","extract":{"type":"conntrack","conntrack":{` +
		`"scheduling":[{"selector":{"SrcK8S_Name":"pod-a"},"heartbeatInterval":"30s"}]}}}]}`
	sections, err := FileSections([]byte(input))
	require.NoError(t, err)
	cfs, err := ParseConfig(&Options{
		PipeLine:   "[]",
		Parameters: string(sections["parameters"]),
	})
	require.NoError(t, err)

	scheduling := cfs.Parameters[0].Extract.ConnTrack.Scheduling
	require.Len(t, scheduling, 1)
	// The case-sensitive field name is preserved (it would be "srck8s_name" if we had used the map form).
	assert.Equal(t, api.Selectors{"SrcK8S_Name": "pod-a"}, scheduling[0].Selector)
}
