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
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"

	ms "github.com/mitchellh/mapstructure"
	pUtils "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	goflowFormat "github.com/netsampler/goflow2/format"
	goflowCommonFormat "github.com/netsampler/goflow2/format/common"
	_ "github.com/netsampler/goflow2/format/protobuf"
	goflowpb "github.com/netsampler/goflow2/pb"
	"github.com/netsampler/goflow2/utils"
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
	in             chan map[string]interface{}
	batchFlushTime time.Duration
	batchMaxLength int
	exitChan       chan bool
}

// TransportWrapper is an implementation of the goflow2 transport interface
type TransportWrapper struct {
	c chan map[string]interface{}
}

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
		return err
	}
	renderedMsg, err := RenderMessage(&message)
	if err == nil {
		w.c <- renderedMsg
	}
	return err
}

// Ingest ingests entries from a network collector using goflow2 library (https://github.com/netsampler/goflow2)
func (r *ingestCollector) Ingest(out chan<- []interface{}) {
	ctx := context.Background()
	r.in = make(chan map[string]interface{}, channelSize)

	// initialize background listeners (a.k.a.netflow+legacy collector)
	r.initCollectorListener(ctx)

	// forever process log lines received by collector
	r.processLogLines(out)

}

func (r *ingestCollector) initCollectorListener(ctx context.Context) {
	transporter := NewWrapper(r.in)
	formatter, err := goflowFormat.FindFormat(ctx, "pb")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		sNF := &utils.StateNetFlow{
			Format:    formatter,
			Transport: transporter,
			Logger:    log.StandardLogger(),
		}

		log.Infof("listening for netflow on host %s, port = %d", r.hostname, r.port)
		err = sNF.FlowRoutine(1, r.hostname, r.port, false)
		log.Fatal(err)

	}()

	go func() {
		sLegacyNF := &utils.StateNFLegacy{
			Format:    formatter,
			Transport: transporter,
			Logger:    log.StandardLogger(),
		}

		log.Infof("listening for legacy netflow on host %s, port = %d", r.hostname, r.port+1)
		err = sLegacyNF.FlowRoutine(1, r.hostname, r.port+1, false)
		log.Fatal(err)
	}()

}

func (r *ingestCollector) processLogLines(out chan<- []interface{}) {
	var records []interface{}
	// Maximum batch time for each batch
	flushRecords := time.NewTicker(r.batchFlushTime)
	defer flushRecords.Stop()
	for {
		select {
		case <-r.exitChan:
			log.Debugf("exiting ingestCollector because of signal")
			return
		case record := <-r.in:
			// TODO: for efficiency, consider forwarding directly as map,
			// as this is reverted back from string to map in later pipeline stages
			recordAsBytes, _ := json.Marshal(record)
			records = append(records, string(recordAsBytes))
			if len(records) >= r.batchMaxLength {
				log.Debugf("ingestCollector sending %d entries", len(records))
				out <- records
				records = []interface{}{}
			}
		case <-flushRecords.C:
			// Process batch of records (if not empty)
			if len(records) > 0 {
				log.Debugf("ingestCollector sending %d entries", len(records))
				out <- records
				records = []interface{}{}
			}
		}
	}
}

// NewIngestCollector create a new ingester
func NewIngestCollector(params api.IngestCollector) (Ingester, error) {
	if params.HostName == "" {
		return nil, fmt.Errorf("ingest hostname not specified")
	}
	if params.Port == 0 {
		return nil, fmt.Errorf("ingest port not specified")
	}

	log.Infof("hostname = %s", params.HostName)
	log.Infof("port = %d", params.Port)

	ch := make(chan bool, 1)
	pUtils.RegisterExitChannel(ch)

	bml := defaultBatchMaxLength
	if params.BatchMaxLen != 0 {
		bml = params.BatchMaxLen
	}

	return &ingestCollector{
		hostname:       params.HostName,
		port:           params.Port,
		exitChan:       ch,
		batchFlushTime: defaultBatchFlushTime,
		batchMaxLength: bml,
	}, nil
}
