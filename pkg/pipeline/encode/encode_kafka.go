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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	defaultReadTimeoutSeconds  = int64(10)
	defaultWriteTimeoutSeconds = int64(10)
)

type kafkaWriteMessage interface {
	WriteMessages(ctx context.Context, msgs ...kafkago.Message) error
}

type encodeKafka struct {
	kafkaParams api.EncodeKafka
	kafkaWriter kafkaWriteMessage
}

// Encode writes entries to kafka topic
func (r *encodeKafka) Encode(in []config.GenericMap) []config.GenericMap {
	log.Debugf("entering encodeKafka Encode, #items = %d", len(in))
	var msgs []kafkago.Message
	msgs = make([]kafkago.Message, 0)
	out := make([]config.GenericMap, 0)
	for _, entry := range in {
		var entryByteArray []byte
		entryByteArray, _ = json.Marshal(entry)
		msg := kafkago.Message{
			Value: entryByteArray,
		}
		msgs = append(msgs, msg)
		out = append(out, entry)
	}
	err := r.kafkaWriter.WriteMessages(context.Background(), msgs...)
	if err != nil {
		log.Errorf("encodeKafka error: %v", err)
	}
	return out
}

// NewEncodeKafka create a new writer to kafka
func NewEncodeKafka(params config.StageParam) (Encoder, error) {
	log.Debugf("entering NewIngestKafka")
	jsonEncodeKafka := params.Encode.Kafka

	var balancer kafkago.Balancer
	switch jsonEncodeKafka.Balancer {
	case api.KafkaEncodeBalancerName("RoundRobin"):
		balancer = &kafkago.RoundRobin{}
	case api.KafkaEncodeBalancerName("LeastBytes"):
		balancer = &kafkago.LeastBytes{}
	case api.KafkaEncodeBalancerName("Hash"):
		balancer = &kafkago.Hash{}
	case api.KafkaEncodeBalancerName("Crc32"):
		balancer = &kafkago.CRC32Balancer{}
	case api.KafkaEncodeBalancerName("Murmur2"):
		balancer = &kafkago.Murmur2Balancer{}
	default:
		balancer = nil
	}

	readTimeoutSecs := defaultReadTimeoutSeconds
	if jsonEncodeKafka.ReadTimeout != 0 {
		readTimeoutSecs = jsonEncodeKafka.ReadTimeout
	}

	writeTimeoutSecs := defaultWriteTimeoutSeconds
	if jsonEncodeKafka.WriteTimeout != 0 {
		writeTimeoutSecs = jsonEncodeKafka.WriteTimeout
	}

	// connect to the kafka server
	kafkaWriter := kafkago.Writer{
		Addr:         kafkago.TCP(jsonEncodeKafka.Address),
		Topic:        jsonEncodeKafka.Topic,
		Balancer:     balancer,
		ReadTimeout:  time.Duration(readTimeoutSecs) * time.Second,
		WriteTimeout: time.Duration(writeTimeoutSecs) * time.Second,
		BatchSize:    jsonEncodeKafka.BatchSize,
		BatchBytes:   jsonEncodeKafka.BatchBytes,
	}

	return &encodeKafka{
		kafkaParams: jsonEncodeKafka,
		kafkaWriter: &kafkaWriter,
	}, nil
}
