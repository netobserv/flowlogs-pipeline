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
	"errors"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type kafkaReadMessage interface {
	ReadMessage(ctx context.Context) (kafkago.Message, error)
	Config() kafkago.ReaderConfig
}

type ingestKafka struct {
	kafkaParams api.IngestKafka
	kafkaReader kafkaReadMessage
	in          chan string
	exitChan    chan bool
	prevRecords []interface{} // copy of most recently sent records; for testing and debugging
}

const channelSizeKafka = 1000
const defaultBatchReadTimeout = int64(100)

// Ingest ingests entries from kafka topic
func (ingestK *ingestKafka) Ingest(out chan<- []interface{}) {
	log.Debugf("entering  ingestKafka.Ingest")

	// initialize background listener
	ingestK.kafkaListener()

	// forever process log lines received by collector
	ingestK.processLogLines(out)
}

// background thread to read kafka messages; place received items into ingestKafka input channel
func (ingestK *ingestKafka) kafkaListener() {
	log.Debugf("entering  kafkaListener")

	go func() {
		for {
			// block until a message arrives
			log.Debugf("before ReadMessage")
			kafkaMessage, err := ingestK.kafkaReader.ReadMessage(context.Background())
			if err != nil {
				log.Errorln(err)
			}
			log.Debugf("string(kafkaMessage) = %s\n", string(kafkaMessage.Value))
			if len(kafkaMessage.Value) > 0 {
				ingestK.in <- string(kafkaMessage.Value)
			}
		}
	}()

}

// read items from ingestKafka input channel, pool them, and send down the pipeline
func (ingestK *ingestKafka) processLogLines(out chan<- []interface{}) {
	var records []interface{}
	duration := time.Duration(ingestK.kafkaParams.BatchReadTimeout) * time.Millisecond
	for {
		select {
		case <-ingestK.exitChan:
			log.Debugf("exiting ingestKafka because of signal")
			return
		case record := <-ingestK.in:
			records = append(records, record)
		case <-time.After(duration): // Maximum batch time for each batch
			// Process batch of records (if not empty)
			if len(records) > 0 {
				log.Debugf("ingestKafka sending %d records", len(records))
				out <- records
				ingestK.prevRecords = records
				log.Debugf("prevRecords = %v", ingestK.prevRecords)
			}
			records = []interface{}{}
		}
	}
}

// NewIngestKafka create a new ingester
func NewIngestKafka(params config.StageParam) (Ingester, error) {
	log.Debugf("entering NewIngestKafka")
	jsonIngestKafka := params.Ingest.Kafka

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
			log.Warningf("groupbalancers parameter missing")
			groupBalancers = append(groupBalancers, &kafkago.RoundRobinGroupBalancer{})
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
		errMsg := "NewIngestKafka: failed to create kafka-go reader"
		log.Errorf("%s", errMsg)
		return nil, errors.New(errMsg)
	}
	log.Debugf("kafkaReader = %v", kafkaReader)

	ch := make(chan bool, 1)
	utils.RegisterExitChannel(ch)

	return &ingestKafka{
		kafkaParams: jsonIngestKafka,
		kafkaReader: kafkaReader,
		exitChan:    ch,
		in:          make(chan string, channelSizeKafka),
		prevRecords: make([]interface{}, 0),
	}, nil
}
