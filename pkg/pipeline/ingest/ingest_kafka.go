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
	"fmt"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/utils"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"time"
)

type ingestKafka struct {
	kafkaParams api.IngestKafka
	kafkaReader *kafkago.Reader
	in          chan string
	exitChan    chan bool
	prevRecords []interface{} // copy of most recently sent records; for testing and debugging
}

const channelSizeKafka = 1000
const defaultBatchReadTimeout = int64(100)

// Ingest ingests entries from kafka topic
func (r *ingestKafka) Ingest(process ProcessFunction) {
	r.in = make(chan string, channelSizeKafka)

	// initialize background listener
	r.kafkaListener()

	// forever process log lines received by collector
	r.processLogLines(process)

}

func (r *ingestKafka) kafkaListener() {
	log.Debugf("entering  kafkaListener")

	go func() {
		var kafkaMessage kafkago.Message
		var err error
		for {
			// block until a message arrives
			log.Debugf("before ReadMessage")
			kafkaMessage, err = r.kafkaReader.ReadMessage(context.Background())
			if err != nil {
				log.Errorln(err)
			}
			log.Debugf("string(kafkaMessage) = %s\n", string(kafkaMessage.Value))
			if len(kafkaMessage.Value) > 0 {
				r.in <- string(kafkaMessage.Value)
			}
		}
	}()

}

func (r *ingestKafka) processLogLines(process ProcessFunction) {
	var records []interface{}
	duration := time.Duration(r.kafkaParams.BatchReadTimeout) * time.Millisecond
	for {
		select {
		case <-r.exitChan:
			log.Debugf("exiting ingestKafka because of signal")
			return
		case record := <-r.in:
			records = append(records, record)
		case <-time.After(duration): // Maximum batch time for each batch
			// Process batch of records (if not empty)
			if len(records) > 0 {
				process(records)
			}
			r.prevRecords = records
			records = []interface{}{}
		}
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
	startOffsetString := jsonIngestKafka.StartOffset
	var startOffset int64
	switch startOffsetString {
	case "FirstOffset", "":
		startOffset = kafkago.FirstOffset
	case "LastOffset":
		startOffset = kafkago.LastOffset
	default:
		startOffset = kafkago.FirstOffset
		log.Errorf("illegal value for StartOffset: %s\n", startOffsetString)
	}
	log.Debugf("startOffset = %v", startOffset)
	groupBalancers := make([]kafkago.GroupBalancer, 0)
	for _, gb := range jsonIngestKafka.GroupBalancers {
		switch gb {
		case "range":
			groupBalancers = append(groupBalancers, &kafkago.RangeGroupBalancer{})
		case "roundRobin":
			groupBalancers = append(groupBalancers, &kafkago.RoundRobinGroupBalancer{})
		case "rackAffinity":
			groupBalancers = append(groupBalancers, &kafkago.RackAffinityGroupBalancer{})
		default:
			return nil, fmt.Errorf("the provided kafka balancer is not valid: %s", gb)
		}
	}

	if jsonIngestKafka.BatchReadTimeout == 0 {
		jsonIngestKafka.BatchReadTimeout = defaultBatchReadTimeout
	}
	log.Infof("BatchReadTimeout = %d", jsonIngestKafka.BatchReadTimeout)

	kafkaReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        jsonIngestKafka.Brokers,
		Topic:          jsonIngestKafka.Topic,
		GroupID:        jsonIngestKafka.GroupId,
		GroupBalancers: groupBalancers,
		StartOffset:    startOffset,
	})
	if kafkaReader == nil {
		errMsg := "NewIngestKafka: failed to create kafkago reader"
		log.Debugf("%s", errMsg)
		return nil, errors.New(errMsg)
	}
	log.Infof("kafkaReader = %v", kafkaReader)

	ch := make(chan bool, 1)
	utils.RegisterExitChannel(ch)

	return &ingestKafka{
		kafkaParams: jsonIngestKafka,
		kafkaReader: kafkaReader,
		exitChan:    ch,
	}, nil
}
