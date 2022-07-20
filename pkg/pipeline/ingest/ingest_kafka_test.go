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
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
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
        batchMaxLen: 1000
        commitInterval: 1000
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
	require.Equal(t, int(500), ingestKafka.batchMaxLength)
	require.Equal(t, time.Duration(500)*time.Millisecond, ingestKafka.kafkaReader.Config().CommitInterval)
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
	require.Equal(t, int(1000), ingestKafka.batchMaxLength)
	require.Equal(t, time.Duration(1000)*time.Millisecond, ingestKafka.kafkaReader.Config().CommitInterval)
}

func removeTimestamp(receivedEntries []config.GenericMap) {
	for _, entry := range receivedEntries {
		delete(entry, "TimeReceived")
	}
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

	// we remove timestamp for test stability
	// Timereceived field is tested in the decodeJson tests
	removeTimestamp(receivedEntries)

	require.Equal(t, 3, len(receivedEntries))
	require.Equal(t, test.DeserializeJSONToMap(t, record1), receivedEntries[0])
	require.Equal(t, test.DeserializeJSONToMap(t, record2), receivedEntries[1])
	require.Equal(t, test.DeserializeJSONToMap(t, record3), receivedEntries[2])
}

type fakeKafkaReader struct {
	readToDo int
	mock.Mock
}

var fakeRecord = []byte(`{"Bytes":20801,"DstAddr":"10.130.2.1","DstPort":36936,"Packets":401,"SrcAddr":"10.130.2.13","SrcPort":3100}`)

// ReadMessage runs in the kafka client thread, which blocks until data is available.
// If data is always available, we have an infinite loop. So we return data only a specified number of time.
func (f *fakeKafkaReader) ReadMessage(ctx context.Context) (kafkago.Message, error) {
	if f.readToDo == 0 {
		// block indefinitely
		c := make(chan struct{})
		<-c
	}
	message := kafkago.Message{
		Topic: "topic1",
		Value: fakeRecord,
	}
	f.readToDo -= 1
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
	fr := fakeKafkaReader{readToDo: 1}
	ingestKafka.kafkaReader = &fr

	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(ingestOutput)
	}()

	// wait for the data to have been processed
	receivedEntries := <-ingestOutput

	// we remove timestamp for test stability
	// Timereceived field is tested in the decodeJson tests
	removeTimestamp(receivedEntries)

	require.Equal(t, 1, len(receivedEntries))
	require.Equal(t, test.DeserializeJSONToMap(t, string(fakeRecord)), receivedEntries[0])
}

func Test_MaxBatchLength(t *testing.T) {
	ingestOutput := make(chan []config.GenericMap)
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)

	// change the ReadMessage function to the mock-up
	fr := fakeKafkaReader{readToDo: 15}
	ingestKafka.kafkaReader = &fr
	ingestKafka.batchMaxLength = 10
	ingestKafka.kafkaParams.BatchReadTimeout = 10000

	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(ingestOutput)
	}()

	// wait for the data to have been processed
	receivedEntries := <-ingestOutput

	require.Equal(t, 10, len(receivedEntries))
}

func Test_BatchTimeout(t *testing.T) {
	ingestOutput := make(chan []config.GenericMap)
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)

	// change the ReadMessage function to the mock-up
	fr := fakeKafkaReader{readToDo: 5}
	ingestKafka.kafkaReader = &fr
	ingestKafka.batchMaxLength = 1000
	ingestKafka.kafkaParams.BatchReadTimeout = 100

	beforeIngest := time.Now()
	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(ingestOutput)
	}()

	require.Equal(t, 0, len(ingestOutput))
	// wait for the data to have been processed
	receivedEntries := <-ingestOutput
	require.Equal(t, 5, len(receivedEntries))

	afterIngest := time.Now()

	// We check that we get entries because of the timer
	// Time must be above timer value but not too much, 20ms is our margin here
	require.LessOrEqual(t, int64(100), afterIngest.Sub(beforeIngest).Milliseconds())
	require.Greater(t, int64(120), afterIngest.Sub(beforeIngest).Milliseconds())
}

func Test_TLSConfigEmpty(t *testing.T) {
	stage := config.NewKafkaPipeline("ingest-kafka", api.IngestKafka{
		Brokers: []string{"any"},
		Topic:   "topic",
		Decoder: api.Decoder{Type: "json"},
	})
	newIngest, err := NewIngestKafka(stage.GetStageParams()[0])
	require.NoError(t, err)
	tlsConfig := newIngest.(*ingestKafka).kafkaReader.Config().Dialer.TLS
	require.Nil(t, tlsConfig)
}

func Test_TLSConfigCA(t *testing.T) {
	ca, cleanup := test.CreateCACert(t)
	defer cleanup()
	stage := config.NewKafkaPipeline("ingest-kafka", api.IngestKafka{
		Brokers: []string{"any"},
		Topic:   "topic",
		Decoder: api.Decoder{Type: "json"},
		TLS: &api.ClientTLS{
			CACertPath: ca,
		},
	})
	newIngest, err := NewIngestKafka(stage.GetStageParams()[0])
	require.NoError(t, err)

	tlsConfig := newIngest.(*ingestKafka).kafkaReader.Config().Dialer.TLS

	require.Empty(t, tlsConfig.Certificates)
	require.NotNil(t, tlsConfig.RootCAs)
	require.Len(t, tlsConfig.RootCAs.Subjects(), 1)
}

func Test_MutualTLSConfig(t *testing.T) {
	ca, user, userKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	stage := config.NewKafkaPipeline("ingest-kafka", api.IngestKafka{
		Brokers: []string{"any"},
		Topic:   "topic",
		Decoder: api.Decoder{Type: "json"},
		TLS: &api.ClientTLS{
			CACertPath:   ca,
			UserCertPath: user,
			UserKeyPath:  userKey,
		},
	})
	newIngest, err := NewIngestKafka(stage.GetStageParams()[0])
	require.NoError(t, err)

	tlsConfig := newIngest.(*ingestKafka).kafkaReader.Config().Dialer.TLS

	require.Len(t, tlsConfig.Certificates, 1)
	require.NotEmpty(t, tlsConfig.Certificates[0].Certificate)
	require.NotNil(t, tlsConfig.Certificates[0].PrivateKey)
	require.NotNil(t, tlsConfig.RootCAs)
	require.Len(t, tlsConfig.RootCAs.Subjects(), 1)
}
