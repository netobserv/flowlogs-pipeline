package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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
