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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestJsonUnmarshalStrict(t *testing.T) {
	type Message struct {
		Foo int    `json:"F"`
		Bar string `json:"B"`
	}
	msg := `{"F":1, "B":"bbb"}`
	var actualMsg Message
	expectedMsg := Message{Foo: 1, Bar: "bbb"}
	err := JsonUnmarshalStrict([]byte(msg), &actualMsg)
	require.NoError(t, err)
	require.Equal(t, expectedMsg, actualMsg)

	msg = `{"F":1, "B":"bbb", "NewField":0}`
	err = JsonUnmarshalStrict([]byte(msg), &actualMsg)
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
