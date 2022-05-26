/*
 * Copyright (C) 2019 IBM, Inc.
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

package pipeline

import (
	"fmt"

	"github.com/heptiolabs/healthcheck"
	"github.com/mariomac/pipes/pkg/graph"
	"github.com/mariomac/pipes/pkg/node"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
)

//TODO: can go into config.Options
const channelBufferLen = 20

// Pipeline manager
type Pipeline struct {
	IsRunning bool
	graph     graph.Graph
}

// NewPipeline defines the pipeline elements
func NewPipeline(options config.Options) (*Pipeline, error) {
	builder := graph.NewBuilder(node.ChannelBufferLen(channelBufferLen))

	graph.RegisterStart(builder, ingest.FileProvider)
	graph.RegisterStart(builder, ingest.CollectorProvider)
	graph.RegisterStart(builder, ingest.GRPCProtobufProvider)
	graph.RegisterStart(builder, ingest.IngestKafkaProvider)

	graph.RegisterMiddle(builder, decode.AwsProvider)
	graph.RegisterMiddle(builder, decode.JSONProvider)
	graph.RegisterMiddle(builder, decode.ProtobufProvider)

	/** etc **/

	graph, err := builder.Build(options.PipeLine)
	if err != nil {
		return nil, err
	}
	return &Pipeline{
		IsRunning: false,
		graph:     graph,
	}, nil
}

func (p *Pipeline) Run() {

	p.IsRunning = true

	p.graph.Run()

	p.IsRunning = false
}

func (p *Pipeline) IsReady() healthcheck.Check {
	return func() error {
		if !p.IsRunning {
			return fmt.Errorf("pipeline is not running")
		}
		return nil
	}
}

func (p *Pipeline) IsAlive() healthcheck.Check {
	return func() error {
		if !p.IsRunning {
			return fmt.Errorf("pipeline is not running")
		}
		return nil
	}
}
