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
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_checkHeader(t *testing.T) {
	wrongHeader := "#wrong_confgen"

	err := checkHeader([]byte(wrongHeader))
	require.Error(t, err)

	err = checkHeader([]byte(definitionHeader))
	require.NoError(t, err)
}

func Test_ParseDefinition(t *testing.T) {
	cg := NewConfGen(&Options{})
	err := cg.ParseDefinition("def", []byte(test.ConfgenNetworkDefBase))
	require.NoError(t, err)
	err = cg.ParseDefinition("def", []byte(test.ConfgenNetworkDefNoAgg))
	require.NoError(t, err)
	err = cg.ParseDefinition("def", []byte(test.ConfgenNetworkDefHisto))
	require.NoError(t, err)
}

func Test_getDefinitionFiles(t *testing.T) {
	filename := "/def.yaml"
	dirPath, err := os.MkdirTemp("", "getDefinitionFilesTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	err = os.WriteFile(filepath.Join(dirPath, filename), []byte(test.ConfgenNetworkDefBase), 0644)
	require.NoError(t, err)
	files := getDefinitionFiles(dirPath)
	require.Equal(t, 1, len(files))
	expected := []string{path.Join(dirPath, filename)}
	require.ElementsMatch(t, expected, files)
}

func Test_RunShortConfGen(t *testing.T) {
	// Prepare
	dirPath, err := os.MkdirTemp("", "RunShortConfGenTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	outDirPath, err := os.MkdirTemp("", "RunShortConfGenTest_out")
	require.NoError(t, err)
	defer os.RemoveAll(outDirPath)

	configOut := filepath.Join(outDirPath, "config.yaml")
	docOut := filepath.Join(outDirPath, "doc.md")
	jsonnetOut := filepath.Join(outDirPath, "jsonnet")
	dashboards := filepath.Join(outDirPath, "dashboards")

	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             configOut,
		DestDocFile:              docOut,
		DestGrafanaJsonnetFolder: jsonnetOut,
		DestDashboardFolder:      dashboards,
		GlobalMetricsPrefix:      "flp_",
	})
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(test.ConfgenShortConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def.yaml"), []byte(test.ConfgenNetworkDefBase), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	destCfgBytes, err := os.ReadFile(configOut)
	require.NoError(t, err)
	var out config.ConfigFileStruct
	err = yaml.UnmarshalStrict(destCfgBytes, &out)
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
		Type: "add_service",
		AddService: &api.NetworkAddServiceRule{
			Input:    "testInput",
			Output:   "testOutput",
			Protocol: "proto",
		},
	}, out.Parameters[1].Transform.Network.Rules[0])

	// Expects aggregates
	require.Len(t, out.Parameters[2].Extract.Aggregates.Rules, 1)
	require.Equal(t, api.AggregateDefinition{
		Name:          "test_aggregates",
		GroupByKeys:   api.AggregateBy{"service"},
		OperationType: "sum",
		OperationKey:  "test_operation_key",
	}, out.Parameters[2].Extract.Aggregates.Rules[0])

	// Expects prom encode
	require.Len(t, out.Parameters[3].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Prefix: "flp_",
		Metrics: api.MetricsItems{{
			Name:     "test_metric",
			Type:     "gauge",
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
			ValueKey: "test_aggregates_value",
			Labels:   []string{"groupByKeys", "aggregate"},
			Remap:    map[string]string{},
			Flatten:  []string{},
			Buckets:  []float64{},
		}},
	}, out.Parameters[3].Encode.Prom)
}

func Test_RunConfGenNoAgg(t *testing.T) {
	// Prepare
	dirPath, err := os.MkdirTemp("", "RunConfGenNoAggTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	outDirPath, err := os.MkdirTemp("", "RunConfGenNoAggTest_out")
	require.NoError(t, err)
	defer os.RemoveAll(outDirPath)

	configOut := filepath.Join(outDirPath, "config.yaml")
	docOut := filepath.Join(outDirPath, "doc.md")
	jsonnetOut := filepath.Join(outDirPath, "jsonnet")
	dashboards := filepath.Join(outDirPath, "dashboards")

	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             configOut,
		DestDocFile:              docOut,
		DestGrafanaJsonnetFolder: jsonnetOut,
		DestDashboardFolder:      dashboards,
	})
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(test.ConfgenShortConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def.yaml"), []byte(test.ConfgenNetworkDefNoAgg), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	destCfgBytes, err := os.ReadFile(configOut)
	require.NoError(t, err)
	var out config.ConfigFileStruct
	err = yaml.Unmarshal(destCfgBytes, &out)
	require.NoError(t, err)
	require.Len(t, out.Pipeline, 3)
	require.Len(t, out.Parameters, 3)

	// Pipeline structure
	require.Equal(t,
		[]config.Stage(
			[]config.Stage{{Name: "ingest_collector"},
				{Name: "transform_network", Follows: "ingest_collector"},
				{Name: "encode_prom", Follows: "transform_network"}}),
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
		Type: "add_service",
		AddService: &api.NetworkAddServiceRule{
			Input:    "testInput",
			Output:   "testOutput",
			Protocol: "proto",
		},
	}, out.Parameters[1].Transform.Network.Rules[0])

	// Expects prom encode
	require.Len(t, out.Parameters[2].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Prefix: "flp_",
		Metrics: api.MetricsItems{{
			Name:     "test_metric",
			Type:     "counter",
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
			ValueKey: "Bytes",
			Labels:   []string{"service"},
			Remap:    map[string]string{},
			Flatten:  []string{},
			Buckets:  []float64{},
		}},
	}, out.Parameters[2].Encode.Prom)
}

func Test_RunLongConfGen(t *testing.T) {
	// Prepare
	dirPath, err := os.MkdirTemp("", "RunLongConfGenTest")
	require.NoError(t, err)
	defer os.RemoveAll(dirPath)
	outDirPath, err := os.MkdirTemp("", "RunLongConfGenTest_out")
	require.NoError(t, err)
	defer os.RemoveAll(outDirPath)

	configOut := filepath.Join(outDirPath, "config.yaml")
	docOut := filepath.Join(outDirPath, "doc.md")
	jsonnetOut := filepath.Join(outDirPath, "jsonnet")
	dashboards := filepath.Join(outDirPath, "dashboards")

	cg := NewConfGen(&Options{
		SrcFolder:                dirPath,
		DestConfFile:             configOut,
		DestDocFile:              docOut,
		DestGrafanaJsonnetFolder: jsonnetOut,
		DestDashboardFolder:      dashboards,
	})
	err = os.WriteFile(filepath.Join(dirPath, configFileName), []byte(test.ConfgenLongConfig), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def1.yaml"), []byte(test.ConfgenNetworkDefBase), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, "def2.yaml"), []byte(test.ConfgenNetworkDefHisto), 0644)
	require.NoError(t, err)

	// Run
	err = cg.Run()
	require.NoError(t, err)

	// Unmarshal output
	destCfgBytes, err := os.ReadFile(configOut)
	require.NoError(t, err)
	var out config.ConfigFileStruct
	err = yaml.UnmarshalStrict(destCfgBytes, &out)
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
	require.Equal(t, api.ReplaceKeys, out.Parameters[1].Transform.Generic.Policy)

	// Expects transform network
	require.Len(t, out.Parameters[2].Transform.Network.Rules, 1)
	require.Equal(t, api.NetworkTransformRule{
		Type: "add_service",
		AddService: &api.NetworkAddServiceRule{
			Input:    "testInput",
			Output:   "testOutput",
			Protocol: "proto",
		},
	}, out.Parameters[2].Transform.Network.Rules[0])

	// Expects aggregates
	require.Len(t, out.Parameters[3].Extract.Aggregates.Rules, 2)
	require.Equal(t, api.AggregateDefinition{
		Name:          "test_aggregates",
		GroupByKeys:   api.AggregateBy{"service"},
		OperationType: "sum",
		OperationKey:  "test_operation_key",
	}, out.Parameters[3].Extract.Aggregates.Rules[0])
	require.Equal(t, api.AggregateDefinition{
		Name:          "test_agg_histo",
		GroupByKeys:   api.AggregateBy{"service"},
		OperationType: "sum",
		OperationKey:  "test_operation_key",
	}, out.Parameters[3].Extract.Aggregates.Rules[1])

	// Expects prom encode; make sure type "histogram" is changed to "agg_histogram"
	require.Len(t, out.Parameters[4].Encode.Prom.Metrics, 2)
	require.Equal(t, &api.PromEncode{
		Prefix: "flp_",
		Metrics: api.MetricsItems{{
			Name:     "test_metric",
			Type:     "gauge",
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
			ValueKey: "test_aggregates_value",
			Labels:   []string{"groupByKeys", "aggregate"},
			Remap:    map[string]string{},
			Flatten:  []string{},
			Buckets:  []float64{},
		}, {
			Name:     "test_histo",
			Type:     "agg_histogram",
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
			ValueKey: "test_aggregates_value",
			Labels:   []string{"groupByKeys", "aggregate"},
			Remap:    map[string]string{},
			Flatten:  []string{},
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
	err := cg.ParseDefinition("defs", []byte(test.ConfgenNetworkDefBase))
	require.NoError(t, err)
	var config Config
	err = yaml.UnmarshalStrict([]byte(test.ConfgenShortConfig), &config)
	require.NoError(t, err)
	cg.SetConfig(&config)

	// Run
	params := cg.GenerateTruncatedConfig()

	require.Len(t, params, 2)
	// Expects aggregates
	require.Len(t, params[0].Extract.Aggregates.Rules, 1)
	require.Equal(t, api.AggregateDefinition{
		Name:          "test_aggregates",
		GroupByKeys:   api.AggregateBy{"service"},
		OperationType: "sum",
		OperationKey:  "test_operation_key",
	}, params[0].Extract.Aggregates.Rules[0])

	// Expects prom encode
	require.Len(t, params[1].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Metrics: api.MetricsItems{{
			Name:     "test_metric",
			Type:     "gauge",
			ValueKey: "test_aggregates_value",
			Labels:   []string{"groupByKeys", "aggregate"},
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
		}},
	}, params[1].Encode.Prom)

	panels, err := cg.GenerateGrafanaJSON()
	require.NoError(t, err)
	var panelJSON map[string]interface{}
	err = yaml.UnmarshalStrict([]byte(panels), &panelJSON)
	require.NoError(t, err)
}

func Test_GenerateTruncatedNoAgg(t *testing.T) {
	// Prepare
	cg := NewConfGen(&Options{
		GenerateStages: []string{"encode_prom"},
	})
	err := cg.ParseDefinition("defs", []byte(test.ConfgenNetworkDefNoAgg))
	require.NoError(t, err)

	// Run
	params := cg.GenerateTruncatedConfig()

	require.Len(t, params, 1)
	// Expects prom encode
	require.Len(t, params[0].Encode.Prom.Metrics, 1)
	require.Equal(t, &api.PromEncode{
		Metrics: api.MetricsItems{{
			Name:     "test_metric",
			Type:     "counter",
			ValueKey: "Bytes",
			Labels:   []string{"service"},
			Filters:  []api.MetricsFilter{{Key: "K", Value: "V"}},
		}},
	}, params[0].Encode.Prom)
}
