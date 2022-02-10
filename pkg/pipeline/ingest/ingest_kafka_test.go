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
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const testConfig1 = `---
log-level: debug
pipeline:
  ingest:
    type: kafka
    kafka:
      brokers: ["1.1.1.1:9092"]
      topic: topic1
      groupid: group1
      startoffset: FirstOffset
      groupbalancers: ["range", "roundRobin"]
      batchReadTimeout: 300
  decode:
    type: json
  transform:
    - type: none
  extract:
    type: none
  encode:
    type: none
  write:
    type: none
`

const testConfig2 = `---
log-level: debug
pipeline:
  ingest:
    type: kafka
    kafka:
      brokers: ["1.1.1.2:9092"]
      topic: topic2
      groupid: group2
      startoffset: LastOffset
      groupbalancers: ["rackAffinity"]
  decode:
    type: none
  transform:
    - type: none
  extract:
    type: none
  encode:
    type: none
  write:
    type: none
`

func initNewIngestKafka(t *testing.T, configTemplate string) Ingester {
	v := test.InitConfig(t, configTemplate)
	val := v.Get("pipeline.ingest.kafka")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)

	config.Opt.PipeLine.Ingest.Kafka = string(b)
	config.Opt.PipeLine.Ingest.Type = "kafka"
	newIngest, err := NewIngestKafka()
	require.Equal(t, err, nil)
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

var receivedEntries []interface{}
var dummyChan chan bool

func dummyProcessFunction(entries []interface{}) {
	receivedEntries = entries
	dummyChan <- true
}

func Test_IngestKafka(t *testing.T) {
	dummyChan = make(chan bool)
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)

	// run Ingest in a separate thread
	go func() {
		ingestKafka.Ingest(dummyProcessFunction)
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
	<-dummyChan

	require.Equal(t, 3, len(receivedEntries))
	require.Equal(t, record1, receivedEntries[0])
	require.Equal(t, record2, receivedEntries[1])
	require.Equal(t, record3, receivedEntries[2])

	// make the ingest thread exit
	ingestKafka.exitChan <- true
	time.Sleep(time.Second)
}
