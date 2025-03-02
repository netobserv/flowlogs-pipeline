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

package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type testMessage struct {
	Elapsed Duration `json:"elapsed" yaml:"elapsed"`
}

func TestDuration_MarshalJSON(t *testing.T) {
	expectedStr := `{"elapsed":"2h0m0s"}`
	msgStr, err := json.Marshal(testMessage{Elapsed: Duration{2 * time.Hour}})
	require.NoError(t, err)
	require.Equal(t, expectedStr, string(msgStr))
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	expectedMsg := testMessage{Elapsed: Duration{2 * time.Hour}}
	var actualMsg testMessage
	require.NoError(t, json.Unmarshal([]byte(`{"elapsed": "2h"}`), &actualMsg))
	require.Equal(t, expectedMsg, actualMsg)
}

func TestDuration_MarshalYAML(t *testing.T) {
	expectedStr := "elapsed: 2h0m0s\n"
	msgStr, err := yaml.Marshal(testMessage{Elapsed: Duration{2 * time.Hour}})
	require.NoError(t, err)
	require.Equal(t, expectedStr, string(msgStr))
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	expectedMsg := testMessage{Elapsed: Duration{2 * time.Hour}}
	var actualMsg testMessage
	require.NoError(t, yaml.UnmarshalStrict([]byte("elapsed: 2h\n"), &actualMsg))
	require.Equal(t, expectedMsg, actualMsg)
}
