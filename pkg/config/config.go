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

package config

import (
	"encoding/json"
	"fmt"

	"github.com/mariomac/pipes/pkg/graph"
	"github.com/mariomac/pipes/pkg/graph/stage"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type GenericMap map[string]interface{}

type Options struct {
	PipeLine PipelineDefinition
	Health   Health
}

type Health struct {
	Port string
}

type PipelineDefinition struct {
	graph.Connector
	// Ingesters
	File        []File
	Collector   []api.IngestCollector
	KafkaIngest []api.IngestKafka
	GRPC        []api.IngestGRPCProto
	// Decoders
	Json     []json.Decoder
	Protobuf []decode.ProtobufConfig
	Aws      []api.DecodeAws
	// Transformers
	Generic []api.TransformGeneric
	Filter  []api.TransformFilter
	Network []api.TransformNetwork
	// Extractors
	Aggregates []api.AggregateDefinition
	// Encoders
	Prom        []api.PromEncode
	KafkaEncode []api.EncodeKafka
	// Writers
	Loki   []api.WriteLoki
	Stdout []api.WriteStdout
}

type File struct {
	stage.Instance
	Type     string // file or file_loop
	Filename string
	Loop     bool
	Chunks   int
}

type Aws struct {
	stage.Instance
	Fields []string
}

func Parse(yamlText []byte) (Options, error) {
	opt := Options{}
	if err := yaml.Unmarshal(yamlText, &opt); err != nil {
		return opt, fmt.Errorf("reading yaml : %w", err)
	}
	return opt, nil
}
