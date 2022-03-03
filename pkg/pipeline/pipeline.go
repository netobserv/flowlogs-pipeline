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
	"os"

	"github.com/heptiolabs/healthcheck"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	log "github.com/sirupsen/logrus"
)

// interface definitions of pipeline components
const (
	StageIngest    = "ingest"
	StageDecode    = "decode"
	StageTransform = "transform"
	StageExtract   = "extract"
	StageEncode    = "encode"
	StageWrite     = "write"
)

// Pipeline manager
type Pipeline struct {
	IsRunning      bool
	pipelineStages []*PipelineEntry
	configStages   []config.Stage
	configParams   []config.StageParam
}

type PipelineEntry struct {
	configStage config.Stage
	stageType   string
	Ingester    ingest.Ingester
	Decoder     decode.Decoder
	Transformer transform.Transformer
	Extractor   extract.Extractor
	Encoder     encode.Encoder
	Writer      write.Writer
	nextStages  []*PipelineEntry
}

var pipelineEntryMap map[string]*PipelineEntry
var firstStage *PipelineEntry

func getIngester(stage config.Stage, params config.StageParam) (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	switch params.Ingest.Type {
	case "file", "file_loop":
		ingester, err = ingest.NewIngestFile(params)
	case "collector":
		ingester, err = ingest.NewIngestCollector(params)
	case "kafka":
		ingester, err = ingest.NewIngestKafka(params)
	default:
		panic(fmt.Sprintf("`ingest` type %s not defined; if no encoder needed, specify `none`", params.Ingest.Type))
	}
	return ingester, err
}

func getDecoder(stage config.Stage, params config.StageParam) (decode.Decoder, error) {
	var decoder decode.Decoder
	var err error
	switch params.Decode.Type {
	case "json":
		decoder, err = decode.NewDecodeJson()
	case "aws":
		decoder, err = decode.NewDecodeAws(params)
	case "none":
		decoder, err = decode.NewDecodeNone()
	default:
		panic(fmt.Sprintf("`decode` type %s not defined; if no encoder needed, specify `none`", params.Decode.Type))
	}
	return decoder, err
}

func getWriter(stage config.Stage, params config.StageParam) (write.Writer, error) {
	var writer write.Writer
	var err error
	switch params.Write.Type {
	case "stdout":
		writer, err = write.NewWriteStdout()
	case "none":
		writer, err = write.NewWriteNone()
	case "loki":
		writer, err = write.NewWriteLoki(params)
	default:
		panic(fmt.Sprintf("`write` type %s not defined; if no encoder needed, specify `none`", params.Write.Type))
	}
	return writer, err
}

func getTransformer(stage config.Stage, params config.StageParam) (transform.Transformer, error) {
	var transformer transform.Transformer
	var err error
	switch params.Transform.Type {
	case transform.OperationGeneric:
		transformer, err = transform.NewTransformGeneric(params)
	case transform.OperationNetwork:
		transformer, err = transform.NewTransformNetwork(params)
	case transform.OperationNone:
		transformer, err = transform.NewTransformNone()
	default:
		panic(fmt.Sprintf("`transform` type %s not defined; if no encoder needed, specify `none`", params.Transform.Type))
	}
	return transformer, err
}

func getExtractor(stage config.Stage, params config.StageParam) (extract.Extractor, error) {
	var extractor extract.Extractor
	var err error
	switch params.Extract.Type {
	case "none":
		extractor, _ = extract.NewExtractNone()
	case "aggregates":
		extractor, err = extract.NewExtractAggregate(params)
	default:
		panic(fmt.Sprintf("`extract` type %s not defined; if no encoder needed, specify `none`", params.Extract.Type))
	}
	return extractor, err
}

func getEncoder(stage config.Stage, params config.StageParam) (encode.Encoder, error) {
	var encoder encode.Encoder
	var err error
	switch params.Encode.Type {
	case "prom":
		encoder, err = encode.NewEncodeProm(params)
	case "kafka":
		encoder, err = encode.NewEncodeKafka(params)
	case "none":
		encoder, _ = encode.NewEncodeNone()
	default:
		panic(fmt.Sprintf("`encode` type %s not defined; if no encoder needed, specify `none`", params.Encode.Type))
	}
	return encoder, err
}

// findStageParameters finds the matching config.param structure and identifies the stage type
func findStageParameters(stage config.Stage, configParams []config.StageParam) (*config.StageParam, string) {
	log.Debugf("findStageParameters: stage = %v", stage)
	for _, param := range configParams {
		log.Debugf("findStageParameters: param = %v", param)
		if stage.Name == param.Name {
			var stageType string
			if param.Ingest.Type != "" {
				stageType = StageIngest
			}
			if param.Decode.Type != "" {
				stageType = StageDecode
			}
			if param.Transform.Type != "" {
				stageType = StageTransform
			}
			if param.Extract.Type != "" {
				stageType = StageExtract
			}
			if param.Encode.Type != "" {
				stageType = StageEncode
			}
			if param.Write.Type != "" {
				stageType = StageWrite
			}
			return &param, stageType
		}
	}
	return nil, ""
}

// NewPipeline defines the pipeline elements
func NewPipeline() (*Pipeline, error) {
	log.Debugf("entering NewPipeline")
	pipelineEntryMap = make(map[string]*PipelineEntry)

	stages := config.PipeLine
	log.Debugf("stages = %v ", stages)
	configParams := config.Parameters
	log.Debugf("configParams = %v ", configParams)
	pipeline := Pipeline{
		pipelineStages: make([]*PipelineEntry, 0),
		configStages:   stages,
		configParams:   configParams,
	}

	for _, stage := range stages {
		log.Debugf("stage = %v", stage)
		params, stageType := findStageParameters(stage, configParams)
		if params == nil {
			err := fmt.Errorf("parameters not defined for stage %s", stage.Name)
			log.Errorf("%v", err)
			return nil, err
		}
		pEntry := PipelineEntry{
			configStage: stage,
			stageType:   stageType,
			nextStages:  make([]*PipelineEntry, 0),
		}
		switch pEntry.stageType {
		case StageIngest:
			pEntry.Ingester, _ = getIngester(stage, *params)
		case StageDecode:
			pEntry.Decoder, _ = getDecoder(stage, *params)
		case StageTransform:
			pEntry.Transformer, _ = getTransformer(stage, *params)
		case StageExtract:
			pEntry.Extractor, _ = getExtractor(stage, *params)
		case StageEncode:
			pEntry.Encoder, _ = getEncoder(stage, *params)
		case StageWrite:
			pEntry.Writer, _ = getWriter(stage, *params)
		}
		pipelineEntryMap[stage.Name] = &pEntry
		pipeline.pipelineStages = append(pipeline.pipelineStages, &pEntry)
		log.Debugf("pipeline = %v", pipeline.pipelineStages)
	}
	log.Debugf("pipeline = %v", pipeline.pipelineStages)

	// connect the stages one to another; find the beginning of the pipeline
	setPipelineFlow(pipeline)

	return &pipeline, nil
}

func setPipelineFlow(pipeline Pipeline) {
	// verify and identify single Ingester. Assume a single ingester and single decoder for now
	for i := range pipeline.pipelineStages {
		stagei := pipeline.pipelineStages[i]
		if stagei.stageType == StageIngest {
			firstStage = stagei
			// verify that no other stages are Ingester
			for j := i + 1; j < len(pipeline.pipelineStages); j++ {
				if pipeline.pipelineStages[j].stageType == StageIngest {
					log.Errorf("only a single ingest stage is allowed")
					os.Exit(1)
				}
			}
		} else {
			// set the follows field
			follows, ok := pipelineEntryMap[stagei.configStage.Follows]
			if !ok {
				log.Errorf("follows stage %s is not yet defined for %s", stagei.configStage.Follows, stagei.configStage.Name)
				os.Exit(1)
			}
			follows.nextStages = append(follows.nextStages, stagei)
			log.Debugf("adding stage pEntry = %v to nextStages of follows = %v", stagei, follows)
		}
	}
	if firstStage == nil {
		log.Errorf("no ingest stage found")
		os.Exit(1)
	}
	log.Debugf("firstStage = %v", firstStage)
	secondStage := firstStage.nextStages[0]
	if len(firstStage.nextStages) != 1 || secondStage.stageType != StageDecode {
		log.Errorf("second stage is not decoder")
	}
}

func (p *Pipeline) Run() {
	p.IsRunning = true
	firstStage.Ingester.Ingest(p.Process)
	p.IsRunning = false
}

func (p Pipeline) invokeStage(stage *PipelineEntry, entries []config.GenericMap) []config.GenericMap {
	log.Debugf("entering invokeStage, stage = %s, type = %s", stage.configStage.Name, stage.stageType)
	var out []config.GenericMap
	switch stage.stageType {
	case StageTransform:
		out = transform.ExecuteTransform(stage.Transformer, entries)
	case StageExtract:
		out = stage.Extractor.Extract(entries)
	case StageEncode:
		out = stage.Encoder.Encode(entries)
	case StageWrite:
		out = stage.Writer.Write(entries)
	}
	return out
}

func (p Pipeline) processStage(stage *PipelineEntry, entries []config.GenericMap) {
	log.Debugf("entering processStage, stage = %s", stage.configStage.Name)
	out := p.invokeStage(stage, entries)
	if len(stage.nextStages) == 0 {
		return
	} else if len(stage.nextStages) == 1 {
		p.processStage(stage.nextStages[0], out)
		return
	}
	for nextStage := range stage.nextStages {
		// make a separate copy of the input data for each following stage
		entriesCopy := make([]config.GenericMap, len(out))
		copy(entriesCopy, out)
		p.processStage(stage.nextStages[nextStage], entriesCopy)
	}
}

// Process is called by the Ingester function
func (p Pipeline) Process(entries []interface{}) {
	log.Debugf("entering pipeline.Process")
	log.Debugf("number of entries = %d", len(entries))
	// already checked first item is an ingester and it has a single follower a decoder
	// Assume for now we have a single decoder
	log.Debugf("firstStage = %v", firstStage)
	secondStage := firstStage.nextStages[0]
	log.Debugf("secondStage = %v", secondStage)
	log.Debugf("number of next stages = %d", len(secondStage.nextStages))
	out := secondStage.Decoder.Decode(entries)
	if len(secondStage.nextStages) == 0 {
		return
	} else if len(secondStage.nextStages) == 1 {
		p.processStage(secondStage.nextStages[0], out)
		return
	}
	log.Debugf(" pipeline.Process, before for loop")
	for nextStage := range secondStage.nextStages {
		// make a separate copy of the input data for each following stage
		entriesCopy := make([]config.GenericMap, len(out))
		copy(entriesCopy, out)
		p.processStage(secondStage.nextStages[nextStage], entriesCopy)
	}
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
