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
	"github.com/stretchr/testify/require"
)

func TestLokiPipeline(t *testing.T) {
	pl := NewCollectorPipeline("ingest", api.IngestCollector{HostName: "127.0.0.1", Port: 9999})
	pl = pl.TransformNetwork("enrich", api.TransformNetwork{Rules: api.NetworkTransformRules{{
		Type:   api.AddKubernetesRuleType,
		Input:  "SrcAddr",
		Output: "SrcK8S",
	}, {
		Type:   api.AddKubernetesRuleType,
		Input:  "DstAddr",
		Output: "DstK8S",
	}}})
	pl = pl.WriteLoki("loki", api.WriteLoki{URL: "http://loki:3100/"})
	stages := pl.GetStages()
	require.Len(t, stages, 3)

	b, err := json.Marshal(stages)
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"ingest"},{"name":"enrich","follows":"ingest"},{"name":"loki","follows":"enrich"}]`, string(b))

	params := pl.GetStageParams()
	require.Len(t, params, 3)

	b, err = json.Marshal(params[0])
	require.NoError(t, err)
	require.Equal(t, `{"name":"ingest","ingest":{"type":"collector","collector":{"hostName":"127.0.0.1","port":9999}}}`, string(b))

	b, err = json.Marshal(params[1])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"enrich","transform":{"type":"network","network":{"directionInfo":{},"rules":[{"input":"SrcAddr","output":"SrcK8S","type":"add_kubernetes"},{"input":"DstAddr","output":"DstK8S","type":"add_kubernetes"}]}}}`, string(b))

	b, err = json.Marshal(params[2])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"loki","write":{"type":"loki","loki":{"url":"http://loki:3100/"}}}`, string(b))
}

func TestGRPCPipeline(t *testing.T) {
	pl := NewGRPCPipeline("grpc", api.IngestGRPCProto{Port: 9050, BufferLen: 50})
	pl = pl.TransformFilter("filter", api.TransformFilter{
		Rules: []api.TransformFilterRule{{
			Type:  "remove_entry_if_doesnt_exist",
			Input: "doesnt_exist",
		}},
	})
	pl = pl.WriteStdout("stdout", api.WriteStdout{Format: "json"})
	stages := pl.GetStages()
	require.Len(t, stages, 3)

	b, err := json.Marshal(stages)
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"grpc"},{"name":"filter","follows":"grpc"},{"name":"stdout","follows":"filter"}]`, string(b))

	params := pl.GetStageParams()
	require.Len(t, params, 3)

	b, err = json.Marshal(params[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"grpc","ingest":{"type":"grpc","grpc":{"port":9050,"bufferLength":50}}}`, string(b))

	b, err = json.Marshal(params[1])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"filter","transform":{"type":"filter","filter":{"rules":[{"input":"doesnt_exist","type":"remove_entry_if_doesnt_exist"}]}}}`, string(b))

	b, err = json.Marshal(params[2])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"stdout","write":{"type":"stdout","stdout":{"format":"json"}}}`, string(b))
}

func TestKafkaPromPipeline(t *testing.T) {
	pl := NewKafkaPipeline("ingest", api.IngestKafka{
		Brokers: []string{"http://kafka"},
		Topic:   "netflows",
		GroupId: "my-group",
		Decoder: api.Decoder{Type: "json"},
		TLS: &api.ClientTLS{
			InsecureSkipVerify: true,
			CACertPath:         "/ca.crt",
		},
	})
	pl = pl.TransformFilter("filter", api.TransformFilter{
		Rules: []api.TransformFilterRule{{
			Type:  "remove_entry_if_doesnt_exist",
			Input: "doesnt_exist",
		}},
	})
	pl = pl.ConnTrack("conntrack", api.ConnTrack{
		KeyDefinition: api.KeyDefinition{},
	})
	pl = pl.Aggregate("aggregate", []api.AggregateDefinition{{
		Name:          "src_as_connection_count",
		GroupByKeys:   api.AggregateBy{"srcAS"},
		OperationType: "count",
	}})
	pl = pl.EncodePrometheus("prom", api.PromEncode{
		Metrics: api.PromMetricsItems{{
			Name: "connections_per_source_as",
			Type: "counter",
			Filter: api.PromMetricsFilter{
				Key:   "name",
				Value: "src_as_connection_count",
			},
			ValueKey: "recent_count",
			Labels:   []string{"by", "aggregate"},
			Buckets:  []float64{},
		}},
		Port:   9090,
		Prefix: "flp_",
	})
	stages := pl.GetStages()
	require.Len(t, stages, 5)

	b, err := json.Marshal(stages)
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"ingest"},{"name":"filter","follows":"ingest"},{"name":"conntrack","follows":"filter"},{"name":"aggregate","follows":"conntrack"},{"name":"prom","follows":"aggregate"}]`, string(b))

	params := pl.GetStageParams()
	require.Len(t, params, 5)

	b, err = json.Marshal(params[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"ingest","ingest":{"type":"kafka","kafka":{"brokers":["http://kafka"],"topic":"netflows","groupid":"my-group","decoder":{"type":"json"},"tls":{"insecureSkipVerify":true,"caCertPath":"/ca.crt"}}}}`, string(b))

	b, err = json.Marshal(params[1])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"filter","transform":{"type":"filter","filter":{"rules":[{"input":"doesnt_exist","type":"remove_entry_if_doesnt_exist"}]}}}`, string(b))

	b, err = json.Marshal(params[2])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"conntrack","extract":{"type":"conntrack","conntrack":{"keyDefinition":{"hash":{}},"tcpFlags":{}}}}`, string(b))

	b, err = json.Marshal(params[3])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"aggregate","extract":{"type":"aggregates","aggregates":[{"name":"src_as_connection_count","groupByKeys":["srcAS"],"operationType":"count"}]}}`, string(b))

	b, err = json.Marshal(params[4])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"prom","encode":{"type":"prom","prom":{"metrics":[{"name":"connections_per_source_as","type":"counter","filter":{"key":"name","value":"src_as_connection_count"},"valueKey":"recent_count","labels":["by","aggregate"],"buckets":[]}],"port":9090,"prefix":"flp_"}}}`, string(b))
}

func TestForkPipeline(t *testing.T) {
	plFork := NewCollectorPipeline("ingest", api.IngestCollector{HostName: "127.0.0.1", Port: 9999})
	plFork.WriteLoki("loki", api.WriteLoki{URL: "http://loki:3100/"})
	plFork.WriteStdout("stdout", api.WriteStdout{})
	stages := plFork.GetStages()
	require.Len(t, stages, 3)

	b, err := json.Marshal(stages)
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"ingest"},{"name":"loki","follows":"ingest"},{"name":"stdout","follows":"ingest"}]`, string(b))

	params := plFork.GetStageParams()
	require.Len(t, params, 3)

	b, err = json.Marshal(params[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"ingest","ingest":{"type":"collector","collector":{"hostName":"127.0.0.1","port":9999}}}`, string(b))

	b, err = json.Marshal(params[1])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"loki","write":{"type":"loki","loki":{"url":"http://loki:3100/"}}}`, string(b))

	b, err = json.Marshal(params[2])
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"stdout","write":{"type":"stdout","stdout":{}}}`, string(b))
}
