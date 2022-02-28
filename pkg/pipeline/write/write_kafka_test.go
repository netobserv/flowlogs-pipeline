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

package write

import (
	"encoding/json"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

const testKafkaConfig = `---
log-level: debug
pipeline:
  write:
    type: kafka
    kafka:
      address: 1.2.3.4:9092
      topic: topic1
`

type fakeKafkaWriter struct {
	mock.Mock
	receivedData []interface{}
}

var receivedData []interface{}

func (f *fakeKafkaWriter) WriteMessages(ctx context.Context, msg ...kafkago.Message) error {
	receivedData = append(receivedData, msg)
	return nil
}

func initNewKafkaWriter(t *testing.T) Writer {
	v := test.InitConfig(t, testKafkaConfig)
	val := v.Get("pipeline.write.kafka")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.NoError(t, err)

	config.Opt.PipeLine.Encode.Kafka = string(b)
	kw, err := NewKafka()
	require.NoError(t, err)
	return kw
}

func Test_EncodeKafka(t *testing.T) {
	kw := initNewKafkaWriter(t)
	encodeKafka := kw.(*Kafka)
	require.Equal(t, "1.2.3.4:9092", encodeKafka.kafkaParams.Address)
	require.Equal(t, "topic1", encodeKafka.kafkaParams.Topic)

	fw := fakeKafkaWriter{receivedData: []interface{}{}}
	encodeKafka.kafkaWriter = &fw

	entry1 := test.GetExtractMockEntry()
	entry2 := test.GetIngestMockEntry(false)
	in := []config.GenericMap{entry1, entry2}
	kw.Write(in)

	var expectedOutputString1 []byte
	var expectedOutputString2 []byte
	expectedOutputString1, _ = json.Marshal(entry1)
	expectedOutputString2, _ = json.Marshal(entry2)
	expectedOutput := []kafkago.Message{
		{
			Value: expectedOutputString1,
		},
		{
			Value: expectedOutputString2,
		},
	}
	require.Equal(t, expectedOutput, fw.receivedData[0])
}
