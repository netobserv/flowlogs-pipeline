/*
 * Copyright (C) 2021 IBM, Inc.
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
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	ms "github.com/mitchellh/mapstructure"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	operationalMetrics "github.com/netobserv/flowlogs-pipeline/pkg/operational/metrics"
	pUtils "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	goflowFormat "github.com/netsampler/goflow2/format"
	goflowCommonFormat "github.com/netsampler/goflow2/format/common"
	_ "github.com/netsampler/goflow2/format/protobuf"
	goflowpb "github.com/netsampler/goflow2/pb"
	"github.com/netsampler/goflow2/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const (
	channelSize           = 1000
	defaultBatchFlushTime = time.Second
	defaultBatchMaxLength = 500
)

type ingestCollector struct {
	hostname       string
	port           int
	portLegacy     int
	in             chan map[string]interface{}
	batchFlushTime time.Duration
	batchMaxLength int
	exitChan       <-chan struct{}
}

// TransportWrapper is an implementation of the goflow2 transport interface
type TransportWrapper struct {
	c chan map[string]interface{}
}

var queueLength = operationalMetrics.NewGauge(prometheus.GaugeOpts{
	Name: "ingest_collector_queue_length",
	Help: "Queue length",
})

var linesProcessed = operationalMetrics.NewCounter(prometheus.CounterOpts{
	Name: "ingest_collector_flow_logs_processed",
	Help: "Number of log lines (flow logs) processed",
})

func NewWrapper(c chan map[string]interface{}) *TransportWrapper {
	tw := TransportWrapper{c: c}
	return &tw
}

func RenderMessage(message *goflowpb.FlowMessage) (map[string]interface{}, error) {
	outputMap := make(map[string]interface{})
	err := ms.Decode(message, &outputMap)
	if err != nil {
		return nil, err
	}
	outputMap["DstAddr"] = goflowCommonFormat.RenderIP(message.DstAddr)
	outputMap["SrcAddr"] = goflowCommonFormat.RenderIP(message.SrcAddr)
	outputMap["DstMac"] = renderMac(message.DstMac)
	outputMap["SrcMac"] = renderMac(message.SrcMac)
	return outputMap, nil
}

func renderMac(macValue uint64) string {
	mac := make([]byte, 8)
	binary.BigEndian.PutUint64(mac, macValue)
	return net.HardwareAddr(mac[2:]).String()
}

func (w *TransportWrapper) Send(_, data []byte) error {
	message := goflowpb.FlowMessage{}
	err := proto.Unmarshal(data, &message)
	if err != nil {
		// temporary fix
		// A PR was submitted to log this error from goflow2:
		// https://github.com/netsampler/goflow2/pull/86
		log.Error(err)
		return err
	}
	renderedMsg, err := RenderMessage(&message)
	if err == nil {
		w.c <- renderedMsg
	}
	return err
}

// Ingest ingests entries from a network collector using goflow2 library (https://github.com/netsampler/goflow2)
func (ingestC *ingestCollector) Ingest(out chan<- []config.GenericMap) {
	ctx := context.Background()
	ingestC.in = make(chan map[string]interface{}, channelSize)

	// initialize background listeners (a.k.a.netflow+legacy collector)
	ingestC.initCollectorListener(ctx)

	// forever process log lines received by collector
	ingestC.processLogLines(out)
}

func (ingestC *ingestCollector) initCollectorListener(ctx context.Context) {
	transporter := NewWrapper(ingestC.in)
	formatter, err := goflowFormat.FindFormat(ctx, "pb")
	if err != nil {
		log.Fatal(err)
	}
	if ingestC.port > 0 {
		go func() {
			sNF := &utils.StateNetFlow{
				Format:    formatter,
				Transport: transporter,
				Logger:    log.StandardLogger(),
			}

			log.Infof("listening for netflow on host %s, port = %d", ingestC.hostname, ingestC.port)
			err = sNF.FlowRoutine(1, ingestC.hostname, ingestC.port, false)
			log.Fatal(err)
		}()
	}

	if ingestC.portLegacy > 0 {
		go func() {
			sLegacyNF := &utils.StateNFLegacy{
				Format:    formatter,
				Transport: transporter,
				Logger:    log.StandardLogger(),
			}

			log.Infof("listening for legacy netflow on host %s, port = %d", ingestC.hostname, ingestC.portLegacy)
			err = sLegacyNF.FlowRoutine(1, ingestC.hostname, ingestC.portLegacy, false)
			log.Fatal(err)
		}()
	}
}

func (ingestC *ingestCollector) processLogLines(out chan<- []config.GenericMap) {
	var records []config.GenericMap
	// Maximum batch time for each batch
	flushRecords := time.NewTicker(ingestC.batchFlushTime)
	defer flushRecords.Stop()
	for {
		select {
		case <-ingestC.exitChan:
			log.Debugf("exiting ingestCollector because of signal")
			return
		case record := <-ingestC.in:
			records = append(records, record)
			if len(records) >= ingestC.batchMaxLength {
				log.Debugf("ingestCollector sending %d entries, %d entries waiting", len(records), len(ingestC.in))
				linesProcessed.Add(float64(len(records)))
				queueLength.Set(float64(len(out)))
				log.Debugf("ingestCollector records = %v", records)
				out <- records
				records = []config.GenericMap{}
			}
		case <-flushRecords.C:
			// Process batch of records (if not empty)
			if len(records) > 0 {
				if len(ingestC.in) > 0 {
					for len(records) < ingestC.batchMaxLength && len(ingestC.in) > 0 {
						record := <-ingestC.in
						records = append(records, record)
					}
				}
				log.Debugf("ingestCollector sending %d entries, %d entries waiting", len(records), len(ingestC.in))
				linesProcessed.Add(float64(len(records)))
				queueLength.Set(float64(len(out)))
				log.Debugf("ingestCollector records = %v", records)
				out <- records
				records = []config.GenericMap{}
			}
		}
	}
}

// NewIngestCollector create a new ingester
func NewIngestCollector(params config.StageParam) (Ingester, error) {
	jsonIngestCollector := api.IngestCollector{}
	if params.Ingest != nil && params.Ingest.Collector != nil {
		jsonIngestCollector = *params.Ingest.Collector
	}
	if jsonIngestCollector.HostName == "" {
		return nil, fmt.Errorf("ingest hostname not specified")
	}
	if jsonIngestCollector.Port == 0 && jsonIngestCollector.PortLegacy == 0 {
		return nil, fmt.Errorf("no ingest port specified")
	}

	log.Infof("hostname = %s", jsonIngestCollector.HostName)
	log.Infof("port = %d", jsonIngestCollector.Port)
	log.Infof("portLegacy = %d", jsonIngestCollector.PortLegacy)

	bml := defaultBatchMaxLength
	if jsonIngestCollector.BatchMaxLen != 0 {
		bml = jsonIngestCollector.BatchMaxLen
	}

	return &ingestCollector{
		hostname:       jsonIngestCollector.HostName,
		port:           jsonIngestCollector.Port,
		portLegacy:     jsonIngestCollector.PortLegacy,
		exitChan:       pUtils.ExitChannel(),
		batchFlushTime: defaultBatchFlushTime,
		batchMaxLength: bml,
	}, nil
}
