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

package ingest

import (
	"encoding/json"
	"errors"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	kafka "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type ingestKafka struct {
	kafkaParams api.IngestKafka
	kafkaReader *kafka.Reader
}

// Ingest ingests entries from kafka topic and sends them down the pipeline
func (r *ingestKafka) Ingest(process ProcessFunction) {
	lines := make([]interface{}, 1)

	for {
		m, err := r.kafkaReader.ReadMessage(context.Background())
		if err != nil {
			log.Errorln(err)
		}
		log.Debugf("message at topic:%v partition:%v offset:%v	%s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		lines[0] = string(m.Value)
		process(lines)
	}
}

// NewIngestKafka create a new ingester
func NewIngestKafka() (Ingester, error) {
	log.Debugf("entering NewIngestKafka")
	ingestKafkaString := config.Opt.PipeLine.Ingest.Kafka
	log.Debugf("ingestKafkaString = %s", ingestKafkaString)
	var jsonIngestKafka api.IngestKafka
	err := json.Unmarshal([]byte(ingestKafkaString), &jsonIngestKafka)
	if err != nil {
		return nil, err
	}

	// connect to the kafka server
	brokers := jsonIngestKafka.Brokers
	topic := jsonIngestKafka.Topic
	groupId := jsonIngestKafka.GroupId
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupId,
	})
	if kafkaReader == nil {
		errMsg := "NewIngestKafka: failed to create kafka reader"
		log.Debugf("%s", errMsg)
		return nil, errors.New(errMsg)
	}

	return &ingestKafka{
		kafkaParams: jsonIngestKafka,
		kafkaReader: kafkaReader,
	}, nil
}
