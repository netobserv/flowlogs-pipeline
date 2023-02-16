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

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
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

var receivedData []kafkago.Message

func (f *fakeKafkaWriter) WriteMessages(_ context.Context, msg ...kafkago.Message) error {
	receivedData = append(receivedData, msg...)
	return nil
}

func initNewEncodeKafka(t *testing.T) Encoder {
	v, cfg := test.InitConfig(t, testKafkaConfig)
	require.NotNil(t, v)

	newEncode, err := NewEncodeKafka(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
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

	entry1 := test.GetExtractMockEntry()
	entry2 := test.GetIngestMockEntry(false)
	newEncode.Encode(entry1)
	newEncode.Encode(entry2)
	expectedOutputString1, _ := json.Marshal(entry1)
	expectedOutputString2, _ := json.Marshal(entry2)
	expectedOutput := []kafkago.Message{
		{Value: expectedOutputString1},
		{Value: expectedOutputString2},
	}
	require.Equal(t, expectedOutput, receivedData)
}

func Test_TLSConfigEmpty(t *testing.T) {
	test.ResetPromRegistry()
	pipeline := config.NewCollectorPipeline("ingest", api.IngestCollector{})
	pipeline.EncodeKafka("encode-kafka", api.EncodeKafka{
		Address: "any",
		Topic:   "topic",
	})
	newEncode, err := NewEncodeKafka(operational.NewMetrics(&config.MetricsSettings{}), pipeline.GetStageParams()[1])
	require.NoError(t, err)
	tlsConfig := newEncode.(*encodeKafka).kafkaWriter.(*kafkago.Writer).Transport.(*kafkago.Transport).TLS
	require.Nil(t, tlsConfig)
}

func Test_TLSConfigCA(t *testing.T) {
	test.ResetPromRegistry()
	ca, cleanup := test.CreateCACert(t)
	defer cleanup()
	pipeline := config.NewCollectorPipeline("ingest", api.IngestCollector{})
	pipeline.EncodeKafka("encode-kafka", api.EncodeKafka{
		Address: "any",
		Topic:   "topic",
		TLS: &api.ClientTLS{
			CACertPath: ca,
		},
	})
	newEncode, err := NewEncodeKafka(operational.NewMetrics(&config.MetricsSettings{}), pipeline.GetStageParams()[1])
	require.NoError(t, err)
	tlsConfig := newEncode.(*encodeKafka).kafkaWriter.(*kafkago.Writer).Transport.(*kafkago.Transport).TLS

	require.Empty(t, tlsConfig.Certificates)
	require.NotNil(t, tlsConfig.RootCAs)
	require.Len(t, tlsConfig.RootCAs.Subjects(), 1) //nolint:staticcheck
}

func Test_MutualTLSConfig(t *testing.T) {
	test.ResetPromRegistry()
	ca, user, userKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	pipeline := config.NewCollectorPipeline("ingest", api.IngestCollector{})
	pipeline.EncodeKafka("encode-kafka", api.EncodeKafka{
		Address: "any",
		Topic:   "topic",
		TLS: &api.ClientTLS{
			CACertPath:   ca,
			UserCertPath: user,
			UserKeyPath:  userKey,
		},
	})
	newEncode, err := NewEncodeKafka(operational.NewMetrics(&config.MetricsSettings{}), pipeline.GetStageParams()[1])
	require.NoError(t, err)

	tlsConfig := newEncode.(*encodeKafka).kafkaWriter.(*kafkago.Writer).Transport.(*kafkago.Transport).TLS

	require.Len(t, tlsConfig.Certificates, 1)
	require.NotEmpty(t, tlsConfig.Certificates[0].Certificate)
	require.NotNil(t, tlsConfig.Certificates[0].PrivateKey)
	require.NotNil(t, tlsConfig.RootCAs)
	require.Len(t, tlsConfig.RootCAs.Subjects(), 1) //nolint:staticcheck
}
