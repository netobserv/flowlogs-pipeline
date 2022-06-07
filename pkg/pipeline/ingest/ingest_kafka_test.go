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

package ingest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

const testConfig1 = `---
log-level: debug
pipeline:
  - name: ingest1
parameters:
  - name: ingest1
    ingest:
      type: kafka
      kafka:
        brokers: ["1.1.1.1:9092"]
        topic: topic1
        groupid: group1
        startOffset: FirstOffset
        groupBalancers: ["range", "roundRobin"]
        batchReadTimeout: 300
        decoder:
          type: json
`

const testConfig2 = `---
log-level: debug
pipeline:
  - name: ingest1
parameters:
  - name: ingest1
    ingest:
      type: kafka
      kafka:
        brokers: ["1.1.1.2:9092"]
        topic: topic2
        groupid: group2
        startOffset: LastOffset
        groupBalancers: ["rackAffinity"]
        decoder:
          type: json
`

func initNewIngestKafka(t *testing.T, configTemplate string) Ingester {
	v := test.InitConfig(t, configTemplate)
	require.NotNil(t, v)

	newIngest, err := NewIngestKafka(config.Parameters[0])
	require.NoError(t, err)
	return newIngest
}

func Test_NewIngestKafka1(t *testing.T) {
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)
	require.Equal(t, "topic1", ingestKafka.kafkaParams.Topic)
	require.Equal(t, "group1", ingestKafka.kafkaParams.GroupId)
	expectedBrokers := []string{"1.1.1.1:9092"}
	require.Equal(t, expectedBrokers, ingestKafka.kafkaParams.Brokers)
	require.Equal(t, "FirstOffset", ingestKafka.kafkaParams.StartOffset)
	require.Equal(t, 2, len(ingestKafka.kafkaReader.Config().GroupBalancers))
	require.Equal(t, int64(300), ingestKafka.kafkaParams.BatchReadTimeout)
}

func Test_NewIngestKafka2(t *testing.T) {
	newIngest := initNewIngestKafka(t, testConfig2)
	ingestKafka := newIngest.(*ingestKafka)
	require.Equal(t, "topic2", ingestKafka.kafkaParams.Topic)
	require.Equal(t, "group2", ingestKafka.kafkaParams.GroupId)
	expectedBrokers := []string{"1.1.1.2:9092"}
	require.Equal(t, expectedBrokers, ingestKafka.kafkaParams.Brokers)
	require.Equal(t, "LastOffset", ingestKafka.kafkaParams.StartOffset)
	require.Equal(t, 1, len(ingestKafka.kafkaReader.Config().GroupBalancers))
	require.Equal(t, defaultBatchReadTimeout, ingestKafka.kafkaParams.BatchReadTimeout)
}

func Test_IngestKafka(t *testing.T) {
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)
	ingestOutput := make(chan []config.GenericMap)

	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(ingestOutput)
	}()
	// wait a second for the ingest pipeline to come up
	time.Sleep(time.Second)

	// feed some data into the pipeline
	record1 := "{\"Bytes\":20801,\"DstAddr\":\"10.130.2.1\",\"DstPort\":36936,\"Packets\":401,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"
	record2 := "{\"Bytes\":20802,\"DstAddr\":\"10.130.2.2\",\"DstPort\":36936,\"Packets\":402,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"
	record3 := "{\"Bytes\":20803,\"DstAddr\":\"10.130.2.3\",\"DstPort\":36936,\"Packets\":403,\"SrcAddr\":\"10.130.2.13\",\"SrcPort\":3100}"

	inChan := ingestKafka.in
	inChan <- record1
	inChan <- record2
	inChan <- record3

	// wait for the data to have been processed
	receivedEntries := <-ingestOutput

	require.Equal(t, 3, len(receivedEntries))
	require.Equal(t, toMap(t, record1), receivedEntries[0])
	require.Equal(t, toMap(t, record2), receivedEntries[1])
	require.Equal(t, toMap(t, record3), receivedEntries[2])
}

func toMap(t *testing.T, in string) config.GenericMap {
	var m config.GenericMap
	err := json.Unmarshal([]byte(in), &m)
	require.NoError(t, err)
	return m
}

type fakeKafkaReader struct {
	mock.Mock
}

var fakeRecord = []byte(`{"Bytes":20801,"DstAddr":"10.130.2.1","DstPort":36936,"Packets":401,"SrcAddr":"10.130.2.13","SrcPort":3100}`)

var performedRead = false

// ReadMessage runs in the kafka client thread, which blocks until data is available.
// If data is always available, we have an infinite loop. So we return data only once.
func (f *fakeKafkaReader) ReadMessage(ctx context.Context) (kafkago.Message, error) {
	if performedRead {
		// block indefinitely
		c := make(chan struct{})
		<-c
	}
	message := kafkago.Message{
		Topic: "topic1",
		Value: fakeRecord,
	}
	performedRead = true
	return message, nil
}

func (f *fakeKafkaReader) Config() kafkago.ReaderConfig {
	return kafkago.ReaderConfig{}
}

func Test_KafkaListener(t *testing.T) {
	ingestOutput := make(chan []config.GenericMap)
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)

	// change the ReadMessage function to the mock-up
	fr := fakeKafkaReader{}
	ingestKafka.kafkaReader = &fr

	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(ingestOutput)
	}()

	// wait for the data to have been processed
	receivedEntries := <-ingestOutput

	require.Equal(t, 1, len(receivedEntries))
	require.Equal(t, toMap(t, string(fakeRecord)), receivedEntries[0])
}
