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
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"

	"github.com/prometheus/client_golang/prometheus"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type kafkaReadMessage interface {
	ReadMessage(ctx context.Context) (kafkago.Message, error)
	Config() kafkago.ReaderConfig
}

type ingestKafka struct {
	kafkaParams    api.IngestKafka
	kafkaReader    kafkaReadMessage
	decoder        decode.Decoder
	in             chan []byte
	exitChan       <-chan struct{}
	batchMaxLength int
	metrics        *metrics
	canLogMessages bool
}

const defaultBatchReadTimeout = int64(1000)
const defaultKafkaBatchMaxLength = 500
const defaultKafkaCommitInterval = 500

// Ingest ingests entries from kafka topic
func (ingestK *ingestKafka) Ingest(out chan<- config.GenericMap) {
	log.Debugf("entering ingestKafka.Ingest")
	ingestK.metrics.createOutQueueLen(out)

	// initialize background listener
	ingestK.kafkaListener()

	// forever process log lines received by collector
	ingestK.processLogLines(out)
}

// background thread to read kafka messages; place received items into ingestKafka input channel
func (ingestK *ingestKafka) kafkaListener() {
	log.Debugf("entering kafkaListener")

	go func() {
		for {
			// block until a message arrives
			log.Debugf("before ReadMessage")
			kafkaMessage, err := ingestK.kafkaReader.ReadMessage(context.Background())
			if err != nil {
				log.Errorln(err)
				continue
			}
			if ingestK.canLogMessages {
				log.Debugf("string(kafkaMessage) = %s\n", string(kafkaMessage.Value))
			}
			messageLen := len(kafkaMessage.Value)
			if messageLen > 0 {
				trafficLabels := prometheus.Labels{
					"type":       ingestK.kafkaParams.Decoder.Type,
					"remote_ip":  ingestK.kafkaParams.Brokers[0],
					"local_ip":   "0.0.0.0",
					"local_port": "0",
				}
				flowTrafficBytesSum.With(trafficLabels).Observe(float64(messageLen))
				ingestK.in <- kafkaMessage.Value
			}
		}
	}()

}

func processRecordDelay(record config.GenericMap) {
	TimeFlowEndInterface, ok := record["TimeFlowEnd"]
	if !ok {
		flowErrors.With(prometheus.Labels{"router": "", "error": "No TimeFlowEnd found"}).Inc()
		return
	}
	TimeFlowEnd, ok := TimeFlowEndInterface.(float64)
	if !ok {
		flowErrors.With(prometheus.Labels{"router": "", "error": "Cannot parse TimeFlowEnd"}).Inc()
		return
	}
	delay := time.Since(time.Unix(int64(TimeFlowEnd), 0)).Seconds()
	processDelaySummary.Observe(delay)
}

func (ingestK *ingestKafka) processRecord(record []byte, out chan<- config.GenericMap) {
	// Decode batch
	decoded, err := ingestK.decoder.Decode(record)
	if err != nil {
		log.WithError(err).Warnf("ignoring flow")
		return
	}

	// Update metrics
	flowDecoderCount.With(
		prometheus.Labels{"worker": "", "name": ingestK.kafkaParams.Decoder.Type}).Inc()
	ingestK.metrics.flowsProcessed.Add(float64(len(record)))

	processRecordDelay(decoded)

	// Send batch
	out <- decoded
}

// read items from ingestKafka input channel, pool them, and send down the pipeline
func (ingestK *ingestKafka) processLogLines(out chan<- config.GenericMap) {
	for {
		select {
		case <-ingestK.exitChan:
			log.Debugf("exiting ingestKafka because of signal")
			return
		case record := <-ingestK.in:
			ingestK.processRecord(record, out)
		}
	}
}

// NewIngestKafka create a new ingester
func NewIngestKafka(opMetrics *operational.Metrics, params config.StageParam) (Ingester, error) {
	log.Debugf("entering NewIngestKafka")
	jsonIngestKafka := api.IngestKafka{}
	if params.Ingest != nil && params.Ingest.Kafka != nil {
		jsonIngestKafka = *params.Ingest.Kafka
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
			log.Warningf("groupbalancers parameter missing")
			groupBalancers = append(groupBalancers, &kafkago.RoundRobinGroupBalancer{})
		}
	}

	if jsonIngestKafka.BatchReadTimeout == 0 {
		jsonIngestKafka.BatchReadTimeout = defaultBatchReadTimeout
	}
	log.Infof("batchReadTimeout = %d", jsonIngestKafka.BatchReadTimeout)

	commitInterval := int64(defaultKafkaCommitInterval)
	if jsonIngestKafka.CommitInterval != 0 {
		commitInterval = jsonIngestKafka.CommitInterval
	}
	log.Infof("commitInterval = %d", jsonIngestKafka.CommitInterval)

	dialer := &kafkago.Dialer{
		Timeout:   kafkago.DefaultDialer.Timeout,
		DualStack: kafkago.DefaultDialer.DualStack,
	}
	if jsonIngestKafka.TLS != nil {
		log.Infof("Using TLS configuration: %v", jsonIngestKafka.TLS)
		tlsConfig, err := jsonIngestKafka.TLS.Build()
		if err != nil {
			return nil, err
		}
		dialer.TLS = tlsConfig
	}

	readerConfig := kafkago.ReaderConfig{
		Brokers:        jsonIngestKafka.Brokers,
		Topic:          jsonIngestKafka.Topic,
		GroupID:        jsonIngestKafka.GroupId,
		GroupBalancers: groupBalancers,
		StartOffset:    startOffset,
		CommitInterval: time.Duration(commitInterval) * time.Millisecond,
		Dialer:         dialer,
	}

	if jsonIngestKafka.PullQueueCapacity > 0 {
		log.Infof("pullQueueCapacity = %d", jsonIngestKafka.PullQueueCapacity)
		readerConfig.QueueCapacity = jsonIngestKafka.PullQueueCapacity
	}

	if jsonIngestKafka.PullMaxBytes > 0 {
		log.Infof("pullMaxBytes = %d", jsonIngestKafka.PullMaxBytes)
		readerConfig.MaxBytes = jsonIngestKafka.PullMaxBytes
	}

	kafkaReader := kafkago.NewReader(readerConfig)
	if kafkaReader == nil {
		errMsg := "NewIngestKafka: failed to create kafka-go reader"
		log.Errorf("%s", errMsg)
		return nil, errors.New(errMsg)
	}
	log.Debugf("kafkaReader = %v", kafkaReader)

	decoder, err := decode.GetDecoder(jsonIngestKafka.Decoder)
	if err != nil {
		return nil, err
	}

	bml := defaultKafkaBatchMaxLength
	if jsonIngestKafka.BatchMaxLen != 0 {
		bml = jsonIngestKafka.BatchMaxLen
	}

	in := make(chan []byte, 2*bml)
	metrics := newMetrics(opMetrics, params.Name, func() int { return len(in) })

	return &ingestKafka{
		kafkaParams:    jsonIngestKafka,
		kafkaReader:    kafkaReader,
		decoder:        decoder,
		exitChan:       utils.ExitChannel(),
		in:             in,
		batchMaxLength: bml,
		metrics:        metrics,
		canLogMessages: jsonIngestKafka.Decoder.Type == api.DecoderName("JSON"),
	}, nil
}
