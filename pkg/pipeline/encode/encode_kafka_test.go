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

package encode

import (
	"encoding/json"
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

const testKafkaConfig = `---
log-level: debug
pipeline:
  encode:
    type: kafka
    kafka:
      addr: 1.2.3.4:9092
      topic: topic1
`

type fakeKafkaWriter struct {
	mock.Mock
}

var receivedData []interface{}

func (f *fakeKafkaWriter) WriteMessages(ctx context.Context, msg ...kafkago.Message) error {
	receivedData = append(receivedData, msg)
	return nil
}

func initNewEncodeKafka(t *testing.T) Encoder {
	v := test.InitConfig(t, testKafkaConfig)
	val := v.Get("pipeline.encode.kafka")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.NoError(t, err)

	config.Opt.PipeLine.Encode.Kafka = string(b)
	newEncode, err := NewEncodeKafka()
	require.NoError(t, err)
	return newEncode
}

func Test_EncodeKafka(t *testing.T) {
	newEncode := initNewEncodeKafka(t)
	encodeKafka := newEncode.(*encodeKafka)
	require.Equal(t, "1.2.3.4:9092", encodeKafka.kafkaParams.Addr)
	require.Equal(t, "topic1", encodeKafka.kafkaParams.Topic)

	fw := fakeKafkaWriter{}
	encodeKafka.kafkaWriter = &fw

	receivedData = make([]interface{}, 0)
	entry1 := test.GetExtractMockEntry()
	entry2 := test.GetIngestMockEntry(false)
	in := []config.GenericMap{entry1, entry2}
	newEncode.Encode(in)
	var expectedOutputString1 []byte
	var expectedOutputString2 []byte
	expectedOutputString1, _ = json.Marshal(entry1)
	expectedOutputString2, _ = json.Marshal(entry2)
	message1 := []kafkago.Message{
		{
			Value: expectedOutputString1,
		},
	}
	message2 := []kafkago.Message{
		{
			Value: expectedOutputString2,
		},
	}
	require.Equal(t, message1, receivedData[0])
	require.Equal(t, message2, receivedData[1])
}
