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
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

const kafkaConfigTemplate = `---
log-level: debug
pipeline:
  - name: kafka_ingest
  - name: decode_json
    follows: kafka_ingest
  - name: transform_generic
    follows: decode_json
  - name: kafka_encode
    follows: transform_generic
  - name: write_none
    follows: kafka_encode
parameters:
  - name: kafka_ingest
    ingest:
      type: kafka
      kafka:
        brokers: ["localhost:9092"]
        topic: topic_in
        groupid: group_test_in
        startoffset: LastOffset
  - name: decode_json
    decode:
      type: json
  - name: transform_generic
    transform:
      type: generic
      generic:
        rules:
          - input: Bytes
            output: k_bytes
          - input: DstAddr
            output: k_dstAddr
          - input: DstPort
            output: k_dstPort
          - input: Packets
            output: k_packets
          - input: SrcAddr
            output: k_srcAddr
          - input: SrcPort
            output: k_srcPort
  - name: kafka_encode
    encode:
      type: kafka
      kafka:
        address: "localhost:9092"
        topic: topic_out
  - name: write_none
    write:
      type: none
`

func Test_KafkaPipeline(t *testing.T) {
	var mainPipeline *Pipeline
	var err error
	v := test.InitConfig(t, kafkaConfigTemplate)
	require.NotNil(t, v)

	mainPipeline, err = NewPipeline()
	require.NoError(t, err)

	// run the pipeline in a separate go-routine
	go func() {
		mainPipeline.Run()
	}()

	// wait for the pipeline to come up
	time.Sleep(time.Second)

	inputData := [][]byte{
		[]byte("{\"Bytes\":20801,\"DstAddr\":\"10.130.2.1\",\"DstPort\":36936,\"Packets\":401,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"),
		[]byte("{\"Bytes\":20802,\"DstAddr\":\"10.130.2.2\",\"DstPort\":36936,\"Packets\":402,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"),
		[]byte("{\"Bytes\":20803,\"DstAddr\":\"10.130.2.3\",\"DstPort\":36936,\"Packets\":403,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"),
	}
	expectedOutputData := [][]byte{
		[]byte("{\"k_bytes\":20801,\"k_dstAddr\":\"10.130.2.1\",\"k_dstPort\":36936,\"k_packets\":401,\"k_srcAddr\":\"10.130.2.13\",\"k_srcPort\":3100}"),
		[]byte("{\"k_bytes\":20802,\"k_dstAddr\":\"10.130.2.2\",\"k_dstPort\":36936,\"k_packets\":402,\"k_srcAddr\":\"10.130.2.13\",\"k_srcPort\":3100}"),
		[]byte("{\"k_bytes\":20803,\"k_dstAddr\":\"10.130.2.3\",\"k_dstPort\":36936,\"k_packets\":403,\"k_srcAddr\":\"10.130.2.13\",\"k_srcPort\":3100}"),
	}

	// prepare a kafka producer on topic_in
	kafkaProducer := kafkago.Writer{
		Addr:  kafkago.TCP("localhost:9092"),
		Topic: "topic_in",
	}
	var msgs []kafkago.Message
	msgs = make([]kafkago.Message, 0)
	for _, entry := range inputData {
		msg := kafkago.Message{
			Value: entry,
		}
		msgs = append(msgs, msg)
	}

	// prepare a kafka consumer on topic_out
	kafkaConsumer := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "topic_out",
	})

	// send data on topic_in to be processed by pipeline
	err = kafkaProducer.WriteMessages(context.Background(), msgs...)
	if err != nil {
		fmt.Printf("error conecting to kafka; cannot perform kafka end-to-end test; err = %v \n", err)
		return
	}

	// read the expected data on topic_out
	var output []byte
	var kafkaMessage kafkago.Message

	for _, entry := range expectedOutputData {
		kafkaMessage, _ = kafkaConsumer.ReadMessage(context.Background())
		output = kafkaMessage.Value
		require.Equal(t, entry, output)
	}
}
