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
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	exitUtils "github.com/netobserv/flowlogs2metrics/pkg/pipeline/utils"
	"net"
	"time"

	ms "github.com/mitchellh/mapstructure"
	goflowFormat "github.com/netsampler/goflow2/format"
	goflowCommonFormat "github.com/netsampler/goflow2/format/common"
	_ "github.com/netsampler/goflow2/format/protobuf"
	goflowpb "github.com/netsampler/goflow2/pb"
	"github.com/netsampler/goflow2/utils"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const channelSize = 1000
const batchMaxTimeInMilliSecs = 1000

type ingestCollector struct {
	hostname string
	port     int
	in       chan map[string]interface{}
	exitChan chan bool
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
func (r *ingestCollector) Ingest(process ProcessFunction) {
	ctx := context.Background()
	r.in = make(chan map[string]interface{}, channelSize)

	// initialize background listeners (a.k.a.netflow+legacy collector)
	r.initCollectorListener(ctx)

	// forever process log lines received by collector
	r.processLogLines(process)

}

func (r *ingestCollector) initCollectorListener(ctx context.Context) {
	transporter := NewWrapper(r.in)
	go func() {
		formatter, err := goflowFormat.FindFormat(ctx, "pb")
		if err != nil {
			log.Fatal(err)
		}
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
		formatter, err := goflowFormat.FindFormat(ctx, "pb")
		if err != nil {
			log.Fatal(err)
		}
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

func (r *ingestCollector) processLogLines(process ProcessFunction) {
	var records []interface{}
	for {
		select {
		case <-r.exitChan:
			log.Debugf("exiting ingestCollector because of signal")
			return
		case record := <-r.in:
			recordAsBytes, _ := json.Marshal(record)
			records = append(records, string(recordAsBytes))
		case <-time.After(time.Millisecond * batchMaxTimeInMilliSecs): // Maximum batch time for each batch
			// Process batch of records (if not empty)
			if len(records) > 0 {
				process(records)
			}
			records = []interface{}{}
		}
	}
}

// NewIngestCollector create a new ingester
func NewIngestCollector() (Ingester, error) {
	ingestCollectorString := config.Opt.PipeLine.Ingest.Collector
	log.Debugf("ingestCollectorString = %s", ingestCollectorString)
	var jsonIngestCollector api.IngestCollector
	err := json.Unmarshal([]byte(ingestCollectorString), &jsonIngestCollector)
	if err != nil {
		return nil, err
	}

	if jsonIngestCollector.HostName == "" {
		return nil, fmt.Errorf("ingest hostname not specified")
	}
	if jsonIngestCollector.Port == 0 {
		return nil, fmt.Errorf("ingest port not specified")
	}

	log.Infof("hostname = %s", jsonIngestCollector.HostName)
	log.Infof("port = %d", jsonIngestCollector.Port)

	ch := make(chan bool, 1)
	exitUtils.RegisterExitChannel(ch)

	return &ingestCollector{
		hostname: jsonIngestCollector.HostName,
		port:     jsonIngestCollector.Port,
		exitChan: ch,
	}, nil
}
