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
	Ingester     ingest.Ingester
	Decoder      decode.Decoder
	Transformers []transform.Transformer
	Writer       write.Writer
	Extractor    extract.Extractor
	Encoder      encode.Encoder
	IsRunning    bool

	start    []*node.Init
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
	case "prom":
		encoder, _ = encode.NewEncodeProm()
	case "json":
		encoder, _ = encode.NewEncodeJson()
	case "kafka":
		encoder, _ = encode.NewEncodeKafka()
	case "none":
		encoder, _ = encode.NewEncodeNone()
	default:
		panic("`encode` not defined; if no encoder needed, specify `none`")
	}
	return encoder, err
}

// NewPipeline defines the pipeline elements
func NewPipeline() (*Pipeline, error) {
	log.Debugf("entering NewPipeline")
	ingester, _ := getIngester()
	decoder, _ := getDecoder()
	transformers, _ := getTransformers()
	writer, _ := getWriter()
	extractor, _ := getExtractor()
	encoder, _ := getEncoder()

	p := &Pipeline{
		Ingester:     ingester,
		Decoder:      decoder,
		Transformers: transformers,
		Extractor:    extractor,
		Encoder:      encoder,
		Writer:       writer,
	}

	// TODO: all this apply* functions can be removed if we change the Ingester, Transformer, etc...
	// interfaces and use them directly here
	ingests := node.AsInit(p.Ingester.Ingest)
	decodes := node.AsMiddle(p.applyDecode)
	transforms := node.AsMiddle(p.applyTransforms)
	writes := node.AsTerminal(p.applyWrite)
	extracts := node.AsMiddle(p.applyExtract)
	encodes := node.AsTerminal(p.applyEncode)

	ingests.SendsTo(decodes)
	decodes.SendsTo(transforms)
	transforms.SendsTo(writes, extracts)
	extracts.SendsTo(encodes)

	p.start = []*node.Init{ingests}
	p.terminal = []*node.Terminal{writes, encodes}

	return p, nil
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

func (p *Pipeline) applyDecode(in <-chan []interface{}, out chan<- []config.GenericMap) {
	for entries := range in {
		out <- p.Decoder.Decode(entries)
	}
}

func (p *Pipeline) applyTransforms(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
	for entries := range in {
		transformed := make([]config.GenericMap, 0, len(entries))
		for _, entry := range entries {
			// TODO: for consistent configurability, each transformer could be a node by itself
			transformed = append(transformed,
				transform.ExecuteTransforms(p.Transformers, entry))
		}
		out <- transformed
	}
}

func (p *Pipeline) applyWrite(in <-chan []config.GenericMap) {
	for entries := range in {
		p.Writer.Write(entries)
	}
}

func (p *Pipeline) applyExtract(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
	for entries := range in {
		out <- p.Extractor.Extract(entries)
	}
}

func (p *Pipeline) applyEncode(in <-chan []config.GenericMap) {
	for entries := range in {
		p.Encoder.Encode(entries)
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
