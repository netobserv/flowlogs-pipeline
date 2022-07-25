/*
 * Copyright (C) 2021 IBM, Inc.
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

package confgen

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_checkHeader(t *testing.T) {
	filename := "/tmp/header.check.txt"
	fakeFilename := "/tmp/fake_file.does.exist"
	wrongHeader := "#wrong_confgen"
	cg := NewConfGen(&Options{})
	err := cg.checkHeader(fakeFilename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(wrongHeader), 0644)
	require.NoError(t, err)
	err = cg.checkHeader(filename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(definitionHeader), 0644)
	require.NoError(t, err)
	err = cg.checkHeader(filename)
	require.NoError(t, err)
}

const generalConfig = `#flp_confgen
description:
  test description
ingest:
  collector:
    port: 2155
    portLegacy: 2156
    hostName: 0.0.0.0
encode:
  type: prom
  prom:
    prefix: flp_
    port: 9102
visualization:
  type: grafana
  grafana:
    dashboards:
      - name: "details"
        title: "Flow-Logs to Metrics - Details"
        time_from: "now-15m"
        tags: "['flp','grafana','dashboard','details']"
        schemaVersion: "16"
`

const networkDefs = `#flp_confgen
description:
  test description
details:
  test details
usage:
  test usage
labels:
  - test
  - label
transform:
  rules:
    - input: testInput
      output: testOutput
      type: add_service
      parameters: proto
extract:
  aggregates:
    - name: test_aggregates
      by:
        - service
      operation: sum
      recordKey: test_record_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: gauge
        valueKey: test_aggregates_value
        labels:
          - by
          - aggregate
visualization:
  type: grafana
  grafana:
  - expr: 'test expression'
    type: graphPanel
    dashboard: test
    title:
      Test grafana title
`

func Test_parseFile(t *testing.T) {
	fakeFilename := "/tmp/fake_file.does.exist"
	filename := "/tmp/parse_file.check.txt"
	cg := NewConfGen(&Options{})
	err := cg.parseFile(fakeFilename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(networkDefs), 0644)
	require.NoError(t, err)
	err = cg.parseFile(filename)
	require.NoError(t, err)
}

func Test_getDefinitionFiles(t *testing.T) {
	dirPath := "/tmp/getDefinitionFilesTest"
	filename := "/def.yaml"
	cg := NewConfGen(&Options{})
	err := os.MkdirAll(dirPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, filename), []byte(networkDefs), 0644)
	require.NoError(t, err)
	files := cg.GetDefinitionFiles(dirPath)
	require.Equal(t, 1, len(files))
	expected := []string{path.Join(dirPath, filename)}
	require.ElementsMatch(t, expected, files)
}

func Test_RunConfGen(t *testing.T) {
	// Prepare
	dirPath := "/tmp/getDefinitionFilesTest"
	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             "/tmp/destConfigTest",
		DestDocFile:              "/tmp/destDocTest",
		DestGrafanaJsonnetFolder: "/tmp/destJsonnetTest",
	})
	err := os.MkdirAll(dirPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(generalConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def.yaml"), []byte(networkDefs), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	type Output struct {
		Pipeline   []config.Stage      `yaml:"pipeline"`
		Parameters []config.StageParam `yaml:"parameters"`
	}
	destCfgBytes, err := ioutil.ReadFile("/tmp/destConfigTest")
	require.NoError(t, err)
	var out Output
	err = yaml.Unmarshal(destCfgBytes, &out)
	require.NoError(t, err)

	// Expects ingest
	require.Equal(t, &api.IngestCollector{
		HostName:   "0.0.0.0",
		Port:       2155,
		PortLegacy: 2156,
	}, out.Parameters[0].Ingest.Collector)

	// Expects transform generic
	require.Equal(t, "replace_keys", out.Parameters[1].Transform.Generic.Policy)

	// Expects transform network
	require.Len(t, out.Parameters[2].Transform.Network.Rules, 1)
	require.Equal(t, api.NetworkTransformRule{
		Input:      "testInput",
		Output:     "testOutput",
		Type:       "add_service",
		Parameters: "proto",
	}, out.Parameters[2].Transform.Network.Rules[0])

	// Expects aggregates
	require.Len(t, out.Parameters[3].Extract.Aggregates, 1)
	require.Equal(t, api.AggregateDefinition{
		Name:      "test_aggregates",
		By:        api.AggregateBy{"service"},
		Operation: "sum",
		RecordKey: "test_record_key",
	}, out.Parameters[3].Extract.Aggregates[0])

	// Expects prom encode
	require.Len(t, out.Parameters[4].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Port:   9102,
		Prefix: "flp_",
		Metrics: api.PromMetricsItems{{
			Name:     "test_metric",
			Type:     "gauge",
			Filter:   api.PromMetricsFilter{Key: "", Value: ""},
			ValueKey: "test_aggregates_value",
			Labels:   []string{"by", "aggregate"},
			Buckets:  []float64{},
		}},
	}, out.Parameters[4].Encode.Prom)
}
