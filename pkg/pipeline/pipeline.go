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
	"errors"
	"fmt"

	"github.com/heptiolabs/healthcheck"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/gopipes/pkg/node"
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
	startNodes    []*node.Init
	terminalNodes []*node.Terminal
	IsRunning     bool
	// TODO: this field is only used for test verification. We should rewrite the build process
	// to be able to remove it from here
	pipelineStages []*pipelineEntry
}

// builder stores the information that is only required during the build of the pipeline
type builder struct {
	pipelineStages   []*pipelineEntry
	configStages     []config.Stage
	configParams     []config.Param
	pipelineEntryMap map[string]*pipelineEntry
	createdStages    map[string]interface{}
	startNodes       []*node.Init
	terminalNodes    []*node.Terminal
}

type pipelineEntry struct {
	configStage config.Stage
	stageType   string
	Ingester    ingest.Ingester
	Decoder     decode.Decoder
	Transformer transform.Transformer
	Extractor   extract.Extractor
	Encoder     encode.Encoder
	Writer      write.Writer
}

func getIngester(params config.Param) (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	switch params.Ingest.Type {
	case "file", "file_loop":
		ingester, err = ingest.NewIngestFile(params)
	case "file_chunks":
		ingester, err = ingest.NewFileChunks(params)
	case "collector":
		ingester, err = ingest.NewIngestCollector(params)
	case "kafka":
		ingester, err = ingest.NewIngestKafka(params)
	default:
		panic(fmt.Sprintf("`ingest` type %s not defined; if no encoder needed, specify `none`", params.Ingest.Type))
	}
	return ingester, err
}

func getDecoder(params config.Param) (decode.Decoder, error) {
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

func getWriter(params config.Param) (write.Writer, error) {
	var writer write.Writer
	var err error
	switch params.Write.Type {
	case "stdout":
		writer, _ = write.NewWriteStdout()
	case "none":
		writer, _ = write.NewWriteNone()
	case "loki":
		writer, _ = write.NewWriteLoki(params)
	default:
		panic(fmt.Sprintf("`write` type %s not defined; if no encoder needed, specify `none`", params.Write.Type))
	}
	return writer, err
}

func getTransformer(params config.Param) (transform.Transformer, error) {
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

func getExtractor(params config.Param) (extract.Extractor, error) {
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

func getEncoder(params config.Param) (encode.Encoder, error) {
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

// find matching config.param structure and identify the stage type
func findStageParameters(stage config.Stage, configParams []config.Param) (*config.Param, string) {
	var param config.Param
	log.Debugf("findStageParameters: stage = %v", stage)
	for index := range configParams {
		param = configParams[index]
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

	stages := config.PipeLine
	log.Debugf("stages = %v ", stages)
	configParams := config.Parameters
	log.Debugf("configParams = %v ", configParams)

	return newBuilder(configParams, stages).build()
}

func newBuilder(params []config.Param, stages []config.Stage) *builder {
	return &builder{
		pipelineEntryMap: map[string]*pipelineEntry{},
		createdStages:    map[string]interface{}{},
		configStages:     stages,
		configParams:     params,
	}
}

// read the configuration stages definition and instantiate the corresponding native Go objects
func (b *builder) readStages() error {
	for _, stage := range b.configStages {
		log.Debugf("stage = %v", stage)
		params, stageType := findStageParameters(stage, b.configParams)
		if params == nil {
			err := fmt.Errorf("parameters not defined for stage %s", stage.Name)
			log.Errorf("%v", err)
			return err
		}
		pEntry := pipelineEntry{
			configStage: stage,
			stageType:   stageType,
		}
		var err error
		switch pEntry.stageType {
		case StageIngest:
			pEntry.Ingester, err = getIngester(*params)
		case StageDecode:
			pEntry.Decoder, err = getDecoder(*params)
		case StageTransform:
			pEntry.Transformer, err = getTransformer(*params)
		case StageExtract:
			pEntry.Extractor, err = getExtractor(*params)
		case StageEncode:
			pEntry.Encoder, err = getEncoder(*params)
		case StageWrite:
			pEntry.Writer, err = getWriter(*params)
		default:
			err = fmt.Errorf("invalid stage type: %v", pEntry.stageType)
		}
		if err != nil {
			return err
		}
		b.pipelineEntryMap[stage.Name] = &pEntry
		b.pipelineStages = append(b.pipelineStages, &pEntry)
		log.Debugf("pipeline = %v", b.pipelineStages)
	}
	log.Debugf("pipeline = %v", b.pipelineStages)
	return nil
}

// reads the configured Go stages and connects between them
func (b *builder) build() (*Pipeline, error) {
	if err := b.readStages(); err != nil {
		return nil, err
	}

	for _, connection := range b.configStages {
		if connection.Name == "" || connection.Follows == "" {
			// ignore entries that do not represent a connection
			continue
		}
		// instantiates (or loads from cache) the destination node of a connection
		dstEntry, ok := b.pipelineEntryMap[connection.Name]
		if !ok {
			return nil, fmt.Errorf("unknown pipeline stage: %s", connection.Name)
		}
		dstNode, err := b.getStageNode(dstEntry)
		if err != nil {
			return nil, err
		}
		dst, ok := dstNode.(node.Receiver)
		if !ok {
			return nil, fmt.Errorf("stage %q of type %q can't receive data",
				connection.Name, dstEntry.stageType)
		}
		// instantiates (or loads from cache) the source node of a connection
		srcEntry, ok := b.pipelineEntryMap[connection.Follows]
		if !ok {
			return nil, fmt.Errorf("unknown pipeline stage: %s", connection.Follows)
		}
		srcNode, err := b.getStageNode(srcEntry)
		if err != nil {
			return nil, err
		}
		src, ok := srcNode.(node.Sender)
		if !ok {
			return nil, fmt.Errorf("stage %q of type %q can't send data",
				connection.Name, dstEntry.stageType)
		}
		log.Debugf("connecting stages: %s --> %s", connection.Follows, connection.Name)

		// connects source and destination node, and catches any panic from the Go-Pipes library.
		var catchErr error
		func() {
			defer func() {
				if msg := recover(); msg != nil {
					catchErr = fmt.Errorf("%q and %q stages haven't compatible input/outputs: %v",
						connection.Follows, connection.Name, msg)
				}
			}()
			src.SendsTo(dst)
		}()
		if catchErr != nil {
			return nil, catchErr
		}
	}

	if len(b.startNodes) == 0 {
		return nil, errors.New("no ingesters have been defined")
	}
	if len(b.terminalNodes) == 0 {
		return nil, errors.New("no writers have been defined")
	}
	return &Pipeline{
		startNodes:     b.startNodes,
		terminalNodes:  b.terminalNodes,
		pipelineStages: b.pipelineStages,
	}, nil
}

func (b *builder) getStageNode(pe *pipelineEntry) (interface{}, error) {
	if stg, ok := b.createdStages[pe.configStage.Name]; ok {
		return stg, nil
	}
	var stage interface{}
	// TODO: modify all the types' interfaces to not need to write loops here, the same
	// as we do with Ingest
	switch pe.stageType {
	case StageIngest:
		init := node.AsInit(pe.Ingester.Ingest)
		b.startNodes = append(b.startNodes, init)
		stage = init
	case StageWrite:
		term := node.AsTerminal(func(in <-chan []config.GenericMap) {
			for i := range in {
				pe.Writer.Write(i)
			}
		})
		b.terminalNodes = append(b.terminalNodes, term)
		stage = term
	case StageDecode:
		stage = node.AsMiddle(func(in <-chan []interface{}, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Decoder.Decode(i)
			}
		})
	case StageEncode:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Encoder.Encode(i)
			}
		})
	case StageTransform:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- transform.ExecuteTransform(pe.Transformer, i)
			}
		})
	case StageExtract:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Extractor.Extract(i)
			}
		})
	default:
		return nil, fmt.Errorf("invalid stage type: %s", pe.stageType)
	}
	b.createdStages[pe.configStage.Name] = stage
	return stage, nil
}

func (p *Pipeline) Run() {
	p.IsRunning = true
	// starting the graph
	for _, s := range p.startNodes {
		s.Start()
	}

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
