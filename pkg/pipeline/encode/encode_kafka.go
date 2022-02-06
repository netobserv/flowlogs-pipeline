/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *	 http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type kafkaWriteMessage interface {
	WriteMessages(ctx context.Context, msg ...kafkago.Message) error
}

type encodeKafka struct {
	kafkaParams api.EncodeKafka
	kafkaWriter kafkaWriteMessage
}

// Encode writes entries to kafka topic
func (r *encodeKafka) Encode(in []config.GenericMap) []interface{} {
	log.Debugf("entering encodeKafka Encode, #items = %d", len(in))
	var msg kafkago.Message
	out := make([]interface{}, 0)
	for _, entry := range in {
		var entryByteArray []byte
		entryByteArray, _ = json.Marshal(entry)
		msg = kafkago.Message{
			Value: entryByteArray,
		}
		err := r.kafkaWriter.WriteMessages(context.Background(), msg)
		if err != nil {
			log.Errorf("encodeKafka error: %v", err)
		}
		out = append(out, entry)
	}
	return out
}

// NewEncodeKafka create a new writer to kafka
func NewEncodeKafka() (Encoder, error) {
	log.Debugf("entering NewIngestKafka")
	encodeKafkaString := config.Opt.PipeLine.Encode.Kafka
	log.Debugf("encodeKafkaString = %s", encodeKafkaString)
	var jsonEncodeKafka api.EncodeKafka
	err := json.Unmarshal([]byte(encodeKafkaString), &jsonEncodeKafka)
	if err != nil {
		return nil, err
	}

	// connect to the kafka server
	kafkaWriter := kafkago.Writer{
		Addr:  kafkago.TCP(jsonEncodeKafka.Addr),
		Topic: jsonEncodeKafka.Topic,
	}

	return &encodeKafka{
		kafkaParams: jsonEncodeKafka,
		kafkaWriter: &kafkaWriter,
	}, nil
}
