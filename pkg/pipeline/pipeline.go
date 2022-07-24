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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/gopipes/pkg/node"
	log "github.com/sirupsen/logrus"
)

// interface definitions of pipeline components
const (
	StageIngest    = "ingest"
	StageTransform = "transform"
	StageExtract   = "extract"
	StageEncode    = "encode"
	StageWrite     = "write"
)

// Pipeline manager
type Pipeline struct {
	startNodes    []*node.Init
	terminalNodes []*node.Terminal
	IsRunning     bool
	// TODO: this field is only used for test verification. We should rewrite the build process
	// to be able to remove it from here
	pipelineStages []*pipelineEntry
}

// NewPipeline defines the pipeline elements
func NewPipeline() (*Pipeline, error) {
	log.Debugf("entering NewPipeline")

	stages := config.PipeLine
	log.Debugf("stages = %v ", stages)
	configParams := config.Parameters
	log.Debugf("configParams = %v ", configParams)

	build := newBuilder(configParams, stages)
	if err := build.readStages(); err != nil {
		return nil, err
	}
	return build.build()
}

func (p *Pipeline) Run() {
	// starting the graph
	for _, s := range p.startNodes {
		s.Start()
	}
	p.IsRunning = true

	// blocking the execution until the graph terminal stages end
	for _, t := range p.terminalNodes {
		<-t.Done()
	}
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
