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
	"github.com/mariomac/go-pipes/pkg/node"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/write"
	log "github.com/sirupsen/logrus"
)

// CfgRoot configuration root path

// interface definitions of pipeline components

// Pipeline manager
type Pipeline struct {
	// todo: remove the fields below, which currently are only accessed from testing
	ingester ingest.Ingester
	decoder  decode.Decoder
	writer   write.Writer

	IsRunning bool

	transformers []transform.Transformer
	// list of nodes that need to be started in order to start the pipeline
	start []*node.Init
	// list of nodes whose finalization need to be checked before considering the whole
	// pipeline as finalized
	terminal []*node.Terminal
}

func getIngester() (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	log.Debugf("entering getIngester, type = %v", config.Opt.PipeLine.Ingest.Type)
	switch config.Opt.PipeLine.Ingest.Type {
	case "file", "file_loop":
		ingester, err = ingest.NewIngestFile()
	case "file_chunks":
		ingester, err = ingest.NewFileChunks()
	case "collector":
		ingester, err = ingest.NewIngestCollector()
	case "kafka":
		ingester, err = ingest.NewIngestKafka()
	default:
		panic("`ingester` not defined")
	}
	return ingester, err
}

func getDecoder() (decode.Decoder, error) {
	var decoder decode.Decoder
	var err error
	switch config.Opt.PipeLine.Decode.Type {
	case "json":
		decoder, err = decode.NewDecodeJson()
	case "aws":
		decoder, err = decode.NewDecodeAws()
	case "none":
		decoder, err = decode.NewDecodeNone()
	default:
		panic("`decode` not defined; if no decoder needed, specify `none`")
	}
	return decoder, err
}

func getTransformers() ([]transform.Transformer, error) {
	return transform.GetTransformers()
}

func getWriter() (write.Writer, error) {
	var writer write.Writer
	var err error
	switch config.Opt.PipeLine.Write.Type {
	case "stdout":
		writer, _ = write.NewWriteStdout()
	case "none":
		writer, _ = write.NewWriteNone()
	case "loki":
		writer, _ = write.NewWriteLoki()
	case "prom":
		writer, _ = write.NewPrometheus()
	case "kafka":
		writer, _ = write.NewKafka()
	default:
		panic("`write` not defined; if no writer needed, specify `none`")
	}
	return writer, err
}

func getExtractor() (extract.Extractor, error) {
	var extractor extract.Extractor
	var err error
	switch config.Opt.PipeLine.Extract.Type {
	case "none":
		extractor, _ = extract.NewExtractNone()
	case "aggregates":
		extractor, _ = extract.NewExtractAggregate()
	default:
		panic("`extract` not defined; if no extractor needed, specify `none`")
	}
	return extractor, err
}

func getEncoder() (encode.Encoder, error) {
	var encoder encode.Encoder
	var err error
	switch config.Opt.PipeLine.Encode.Type {
	case "json":
		encoder, _ = encode.NewEncodeJson()
	case "none":
		encoder, _ = encode.NewEncodeNone()
	default:
		panic("`encode` not defined; if no encoder needed, specify `none`")
	}
	return encoder, err
}

// NewPipeline defines the pipeline elements
func NewPipeline() *Pipeline {
	log.Debugf("entering NewPipeline")
	ingester, _ := getIngester()
	decoder, _ := getDecoder()
	transformers, _ := getTransformers()
	writer, _ := getWriter()
	extractor, _ := getExtractor()
	encoder, _ := getEncoder()
	// TODO: enable encoders and extracts when we enable the "flexible" pipeline
	_, _ = extractor, encoder

	ingests := node.AsInit(ingester.Ingest)
	decodes := node.AsMiddle(decodeLoop(decoder))
	transforms := node.AsMiddle(transformLoop(transformers))
	writes := node.AsTerminal(writeLoop(writer))
	//encodes := node.AsMiddle(encoder.Encode)
	//extracts := node.AsMiddle(extractor.Extract)

	ingests.SendsTo(decodes)
	decodes.SendsTo(transforms)
	transforms.SendsTo(writes /*, extracts*/)
	//extracts.SendsTo(encodes.Encode)

	return &Pipeline{
		transformers: transformers,
		start:        []*node.Init{ingests},
		terminal:     []*node.Terminal{writes},
		ingester:     ingester,
		decoder:      decoder,
		writer:       writer,
	}
}

func (p *Pipeline) Run() {
	p.IsRunning = true
	p.process()
	p.IsRunning = false
}

// Process is builds the processing graph and waits until it finishes
func (p Pipeline) process() {
	log.Debugf("entering pipeline.Process")

	// starting the graph
	for _, s := range p.start {
		s.Start()
	}

	// blocking the execution until the graph terminal stages to end before
	for _, t := range p.terminal {
		<-t.Done()
	}
}

func transformLoop(
	transformers []transform.Transformer,
) func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
	return func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
		for entries := range in {
			transformed := make([]config.GenericMap, 0, len(entries))
			for _, entry := range entries {
				// TODO: for consistent configurability, each transformer could be a node by itself
				transformed = append(transformed,
					transform.ExecuteTransforms(transformers, entry))
			}
			out <- transformed
		}
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

func decodeLoop(d decode.Decoder) func(in <-chan []interface{}, out chan<- []config.GenericMap) {
	return func(in <-chan []interface{}, out chan<- []config.GenericMap) {
		for i := range in {
			out <- d.Decode(i)
		}
	}
}

func writeLoop(w write.Writer) func(in <-chan []config.GenericMap) {
	return func(in <-chan []config.GenericMap) {
		for i := range in {
			w.Write(i)
		}
	}
}
