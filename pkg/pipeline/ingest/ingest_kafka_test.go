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
      batchReadTimeout: 30
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
	newIngest, err := NewIngestKafka()
	require.Equal(t, err, nil)
	return newIngest
}

func Test_NewIngestKafka(t *testing.T) {
	newIngest := initNewIngestKafka(t, testConfig1)
	ingestKafka := newIngest.(*ingestKafka)
	require.Equal(t, "topic1", ingestKafka.kafkaParams.Topic)
	require.Equal(t, "group1", ingestKafka.kafkaParams.GroupId)
	expectedBrokers := []string{"1.1.1.1:9092"}
	require.Equal(t, expectedBrokers, ingestKafka.kafkaParams.Brokers)
	require.Equal(t, "FirstOffset", ingestKafka.kafkaParams.StartOffset)
	require.Equal(t, 2, len(ingestKafka.kafkaReader.Config().GroupBalancers))
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
}
