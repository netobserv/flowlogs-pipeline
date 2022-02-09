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
}

func getIngester() (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	log.Debugf("entering getIngester, type = %v", config.Opt.PipeLine.Ingest.Type)
	switch config.Opt.PipeLine.Ingest.Type {
	case "file", "file_loop":
		ingester, err = ingest.NewIngestFile()
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

	return p, nil
}

func (p *Pipeline) Run() {
	p.Ingester.Ingest(p.Process)
}

// Process is called by the Ingester function
func (p Pipeline) Process(entries []interface{}) {
	log.Debugf("entering pipeline.Process")
	log.Debugf("number of entries = %d", len(entries))
	decoded := p.Decoder.Decode(entries)
	transformed := make([]config.GenericMap, 0)
	var flowEntry config.GenericMap
	for _, entry := range decoded {
		flowEntry = transform.ExecuteTransforms(p.Transformers, entry)
		transformed = append(transformed, flowEntry)
	}

	_ = p.Writer.Write(transformed)

	extracted := p.Extractor.Extract(transformed)
	_ = p.Encoder.Encode(extracted)
}
