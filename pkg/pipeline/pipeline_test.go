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
	"testing"

	"github.com/sirupsen/logrus"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/write"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/stretchr/testify/require"
)

func Test_transformToLoki(t *testing.T) {
	var transformed []config.GenericMap
	input := config.GenericMap{"key": "value"}
	transform, err := transform.NewTransformNone()
	require.NoError(t, err)
	transformed = append(transformed, transform.Transform(input))

	config.Opt.PipeLine.Write.Loki = "{}"
	loki, err := write.NewWriteLoki()
	loki.Write(transformed)
	require.NoError(t, err)
}

const configTemplate = `---
log-level: debug
pipeline:
  ingest:
    type: file
    file:
      filename: ../../hack/examples/ocp-ipfix-flowlogs.json
  decode:
    type: json
  transform:
    - type: generic
      generic:
        rules:
          - input: Bytes
            output: fl2m_bytes
          - input: DstAddr
            output: fl2m_dstAddr
          - input: DstPort
            output: fl2m_dstPort
          - input: Packets
            output: fl2m_packets
          - input: SrcAddr
            output: fl2m_srcAddr
          - input: SrcPort
            output: fl2m_srcPort
  extract:
    type: none
  encode:
    type: none
  write:
    type: none
`

func Test_SimplePipeline(t *testing.T) {
	loadGlobalConfig(t)

	mainPipeline, err := NewPipeline()
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()

	// What is there left to check? Check length of saved data of each stage in private structure.
	ingester := mainPipeline.Ingester.(*ingest.IngestFile)
	decoder := mainPipeline.Decoder.(*decode.DecodeJson)
	writer := mainPipeline.Writer.(*write.WriteNone)
	require.Equal(t, 5103, len(ingester.PrevRecords))
	require.Equal(t, len(ingester.PrevRecords), len(decoder.PrevRecords))
	require.Equal(t, len(ingester.PrevRecords), len(writer.PrevRecords))

	// checking that the processing is done for at least the first line of the logs
	require.Equal(t, ingester.PrevRecords[0], decoder.PrevRecords[0])
	// values checked from the first line of the ../../hack/examples/ocp-ipfix-flowlogs.json file
	require.Equal(t, config.GenericMap{
		"fl2m_bytes":   float64(20800),
		"fl2m_dstAddr": "10.130.2.2",
		"fl2m_dstPort": float64(36936),
		"fl2m_packets": float64(400),
		"fl2m_srcAddr": "10.130.2.13",
		"fl2m_srcPort": float64(3100),
	}, writer.PrevRecords[0])
}

func BenchmarkPipeline(b *testing.B) {
	logrus.StandardLogger().SetLevel(logrus.ErrorLevel)
	t := &testing.T{}
	loadGlobalConfig(t)
	config.Opt.PipeLine.Ingest.Type = "file_chunks"
	if t.Failed() {
		b.Fatalf("unexpected error loading config")
	}
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		p, err := NewPipeline()
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		b.StartTimer()
		p.Run()
	}
}

func loadGlobalConfig(t *testing.T) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	v := test.InitConfig(t, configTemplate)
	config.Opt.PipeLine.Ingest.Type = "file"
	config.Opt.PipeLine.Decode.Type = "json"
	config.Opt.PipeLine.Extract.Type = "none"
	config.Opt.PipeLine.Encode.Type = "none"
	config.Opt.PipeLine.Write.Type = "none"
	config.Opt.PipeLine.Ingest.File.Filename = "../../hack/examples/ocp-ipfix-flowlogs.json"

	val := v.Get("pipeline.transform")
	b, err := json.Marshal(&val)
	require.NoError(t, err)
	config.Opt.PipeLine.Transform = string(b)
}
