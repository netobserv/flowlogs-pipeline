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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

const testKafkaConfig = `---
log-level: debug
pipeline:
  - name: encode1
parameters:
  - name: encode1
    encode:
      type: kafka
      kafka:
        address: 1.2.3.4:9092
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
	require.NotNil(t, v)

	newEncode, err := NewEncodeKafka(config.Parameters[0])
	require.NoError(t, err)
	return newEncode
}

func Test_EncodeKafka(t *testing.T) {
	newEncode := initNewEncodeKafka(t)
	encodeKafka := newEncode.(*encodeKafka)
	require.Equal(t, "1.2.3.4:9092", encodeKafka.kafkaParams.Address)
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
	expectedOutput := []kafkago.Message{
		{
			Value: expectedOutputString1,
		},
		{
			Value: expectedOutputString2,
		},
	}
	require.Equal(t, expectedOutput, receivedData[0])
}
