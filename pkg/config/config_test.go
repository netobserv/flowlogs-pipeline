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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	var cfs ConfigFileStruct
	err := yaml.Unmarshal([]byte(cfg), &cfs)
	require.NoError(t, err)
	require.Equal(t, "netobserv_", cfs.MetricsSettings.Prefix)
	require.Equal(t, 9102, cfs.MetricsSettings.PromConnectionInfo.Port)

	err = json.Unmarshal([]byte(cfg), &cfs)
	require.NoError(t, err)
	require.Equal(t, "netobserv_", cfs.MetricsSettings.Prefix)
	require.Equal(t, 9102, cfs.MetricsSettings.PromConnectionInfo.Port)
}

func TestUnmarshalFromViper(t *testing.T) {
	// Make sure unmarshaling from Viper works; the trick is viper uses lower-case keys,
	// which works with JSON unmarshaler as it's case insensitive, but not with YAML unmarshaler
	input := `{"parameters":[{"name":"loki","write":{"type":"loki","loki":{"tenantID":"netobserv","clientConfig":{"proxy_url":null}}}}]}`
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader([]byte(input))
	err := v.ReadConfig(r)
	require.NoError(t, err)
	str := v.Get("parameters")
	b, err := json.Marshal(str)
	require.NoError(t, err)
	cfs, err := ParseConfig(&Options{
		PipeLine:   "[]",
		Parameters: string(b),
	})
	require.NoError(t, err)
	assert.Equal(t, "netobserv", cfs.Parameters[0].Write.Loki.TenantID)
}
