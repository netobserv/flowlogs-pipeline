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
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_checkHeader(t *testing.T) {
	wrongHeader := "#wrong_confgen"

	err := checkHeader([]byte(wrongHeader))
	require.Error(t, err)

	err = checkHeader([]byte(definitionHeader))
	require.NoError(t, err)
}

const shortConfig = `#flp_confgen
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

const longConfig = `#flp_confgen
description:
  test description
ingest:
  collector:
    port: 2155
    portLegacy: 2156
    hostName: 0.0.0.0
transform:
  generic:
    rules:
    - input: SrcAddr
      output: srcIP
encode:
  type: prom
  prom:
    prefix: flp_
    port: 9102
write:
  type: loki
  loki:
    url: http://loki:3100
    staticLabels:
      job: flowlogs-pipeline
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

func Test_ParseDefinition(t *testing.T) {
	cg := NewConfGen(&Options{})
	err := cg.ParseDefinition("def", []byte(networkDefs))
	require.NoError(t, err)
}

func Test_getDefinitionFiles(t *testing.T) {
	filename := "/def.yaml"
	dirPath, err := ioutil.TempDir("", "getDefinitionFilesTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	err = os.WriteFile(filepath.Join(dirPath, filename), []byte(networkDefs), 0644)
	require.NoError(t, err)
	files := getDefinitionFiles(dirPath)
	require.Equal(t, 1, len(files))
	expected := []string{path.Join(dirPath, filename)}
	require.ElementsMatch(t, expected, files)
}

func Test_RunShortConfGen(t *testing.T) {
	// Prepare
	dirPath, err := ioutil.TempDir("", "RunShortConfGenTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	outDirPath, err := ioutil.TempDir("", "RunShortConfGenTest_out")
	require.NoError(t, err)
	defer os.RemoveAll(outDirPath)

	configOut := filepath.Join(outDirPath, "config.yaml")
	docOut := filepath.Join(outDirPath, "doc.md")
	jsonnetOut := filepath.Join(outDirPath, "jsonnet")

	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             configOut,
		DestDocFile:              docOut,
		DestGrafanaJsonnetFolder: jsonnetOut,
	})
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(shortConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def.yaml"), []byte(networkDefs), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	destCfgBytes, err := ioutil.ReadFile(configOut)
	require.NoError(t, err)
	var out config.ConfigFileStruct
	err = yaml.Unmarshal(destCfgBytes, &out)
	require.NoError(t, err)
	require.Len(t, out.Pipeline, 4)
	require.Len(t, out.Parameters, 4)

	// Pipeline structure
	require.Equal(t,
		[]config.Stage(
			[]config.Stage{{Name: "ingest_collector"},
				{Name: "transform_network", Follows: "ingest_collector"},
				{Name: "extract_aggregate", Follows: "transform_network"},
				{Name: "encode_prom", Follows: "extract_aggregate"}}),
		out.Pipeline,
	)

	// Expects ingest
	require.Equal(t, &api.IngestCollector{
		HostName:   "0.0.0.0",
		Port:       2155,
		PortLegacy: 2156,
	}, out.Parameters[0].Ingest.Collector)

	// Expects transform network
	require.Len(t, out.Parameters[1].Transform.Network.Rules, 1)
	require.Equal(t, api.NetworkTransformRule{
		Input:      "testInput",
		Output:     "testOutput",
		Type:       "add_service",
		Parameters: "proto",
	}, out.Parameters[1].Transform.Network.Rules[0])

	// Expects aggregates
	require.Len(t, out.Parameters[2].Extract.Aggregates, 1)
	require.Equal(t, api.AggregateDefinition{
		Name:      "test_aggregates",
		By:        api.AggregateBy{"service"},
		Operation: "sum",
		RecordKey: "test_record_key",
	}, out.Parameters[2].Extract.Aggregates[0])

	// Expects prom encode
	require.Len(t, out.Parameters[3].Encode.Prom.Metrics, 1)
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
	}, out.Parameters[3].Encode.Prom)
}

func Test_RunLongConfGen(t *testing.T) {
	// Prepare
	dirPath, err := ioutil.TempDir("", "RunLongConfGenTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	outDirPath, err := ioutil.TempDir("", "RunLongConfGenTest_out")
	require.NoError(t, err)
	defer os.RemoveAll(outDirPath)

	configOut := filepath.Join(outDirPath, "config.yaml")
	docOut := filepath.Join(outDirPath, "doc.md")
	jsonnetOut := filepath.Join(outDirPath, "jsonnet")

	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             configOut,
		DestDocFile:              docOut,
		DestGrafanaJsonnetFolder: jsonnetOut,
	})
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(longConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def.yaml"), []byte(networkDefs), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	destCfgBytes, err := ioutil.ReadFile(configOut)
	require.NoError(t, err)
	var out config.ConfigFileStruct
	err = yaml.Unmarshal(destCfgBytes, &out)
	require.NoError(t, err)
	require.Len(t, out.Parameters, 6)
	require.Len(t, out.Pipeline, 6)

	// Pipeline structure
	require.Equal(t,
		[]config.Stage(
			[]config.Stage{{Name: "ingest_collector"},
				{Name: "transform_generic", Follows: "ingest_collector"},
				{Name: "transform_network", Follows: "transform_generic"},
				{Name: "extract_aggregate", Follows: "transform_network"},
				{Name: "encode_prom", Follows: "extract_aggregate"},
				{Name: "write_loki", Follows: "transform_network"}}),
		out.Pipeline,
	)

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

	// Expects loki
	require.Equal(t, &api.WriteLoki{
		URL:          "http://loki:3100",
		StaticLabels: model.LabelSet{"job": "flowlogs-pipeline"},
	}, out.Parameters[5].Write.Loki)
}

func Test_GenerateTruncatedConfig(t *testing.T) {
	// Prepare
	cg := NewConfGen(&Options{
		GenerateStages: []string{"extract_aggregate", "encode_prom"},
	})
	err := cg.ParseDefinition("defs", []byte(networkDefs))
	require.NoError(t, err)

	// Run
	params := cg.GenerateTruncatedConfig()

	require.Len(t, params, 2)
	// Expects aggregates
	require.Len(t, params[0].Extract.Aggregates, 1)
	require.Equal(t, api.AggregateDefinition{
		Name:      "test_aggregates",
		By:        api.AggregateBy{"service"},
		Operation: "sum",
		RecordKey: "test_record_key",
	}, params[0].Extract.Aggregates[0])

	// Expects prom encode
	require.Len(t, params[1].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Metrics: api.PromMetricsItems{{
			Name:     "test_metric",
			Type:     "gauge",
			Filter:   api.PromMetricsFilter{Key: "", Value: ""},
			ValueKey: "test_aggregates_value",
			Labels:   []string{"by", "aggregate"},
		}},
	}, params[1].Encode.Prom)
}
