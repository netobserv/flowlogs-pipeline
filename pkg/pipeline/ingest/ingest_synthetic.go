/*
 * Copyright (C) 2023 IBM, Inc.
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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type IngestSynthetic struct {
	params           api.IngestSynthetic
	exitChan         <-chan struct{}
	metricsProcessed prometheus.Counter
}

const (
	defaultConnections    = 100
	defaultBatchLen       = 10
	defaultFlowLogsPerMin = 2000
)

var (
	flowLogsProcessed = operational.DefineMetric(
		"ingest_synthetic_flows_processed",
		"Number of flow logs processed",
		operational.TypeCounter,
		"stage",
	)
)

// Ingest generates flow logs according to provided parameters
func (ingestS *IngestSynthetic) Ingest(out chan<- config.GenericMap) {
	log.Debugf("entering IngestSynthetic Ingest, params = %v", ingestS.params)
	// get a list of flow log entries, one per desired connection
	// these flow logs will be sent again and again to simulate ongoing traffic on those connections
	flowLogs := utils.GenerateConnectionFlowEntries(ingestS.params.Connections)
	nLogs := len(flowLogs)
	next := 0

	// compute time interval between batches; divide BatchMaxLen by FlowLogsPerMin and adjust the types
	ticker := time.NewTicker(time.Duration(int(time.Minute*time.Duration(ingestS.params.BatchMaxLen)) / ingestS.params.FlowLogsPerMin))

	// loop forever
	// on each iteration send BatchMaxLen flow logs from the array of flow logs.
	// if necessary, send the contents of the flowLogs array multiple times (in sub-batches) to fill the number of BatchMaxLen flow logs needed
	for {
		select {
		case <-ingestS.exitChan:
			log.Debugf("exiting IngestSynthetic because of signal")
			return
		case <-ticker.C:
			// flowsLeft designates the number out of BatchMaxLen that must still be sent in this batch
			// next designates the next flow log entry to be sent
			// remainder designates how many flow logs remain in the flowLogs array that can be sent on the next sub-batch.
			flowsLeft := ingestS.params.BatchMaxLen
			log.Debugf("flowsLeft = %d", flowsLeft)
			subBatchLen := flowsLeft
			for flowsLeft > 0 {
				remainder := nLogs - next
				if subBatchLen > remainder {
					subBatchLen = remainder
				}
				log.Debugf("flowsLeft = %d, remainder = %d, subBatchLen = %d", flowsLeft, remainder, subBatchLen)
				subBatch := flowLogs[next : next+subBatchLen]
				ingestS.sendBatch(subBatch, out)
				ingestS.metricsProcessed.Add(float64(subBatchLen))
				flowsLeft -= subBatchLen
				next += subBatchLen
				if subBatchLen == remainder {
					next = 0
					subBatchLen = flowsLeft
				}
			}
		}
	}
}

func (ingestS *IngestSynthetic) sendBatch(flows []config.GenericMap, out chan<- config.GenericMap) {
	for _, flow := range flows {
		out <- flow
	}
}

// NewIngestSynthetic create a new ingester
func NewIngestSynthetic(opMetrics *operational.Metrics, params config.StageParam) (Ingester, error) {
	log.Debugf("entering NewIngestSynthetic")
	confIngestSynthetic := api.IngestSynthetic{}
	if params.Ingest != nil || params.Ingest.Synthetic != nil {
		confIngestSynthetic = *params.Ingest.Synthetic
	}
	if confIngestSynthetic.Connections == 0 {
		confIngestSynthetic.Connections = defaultConnections
	}
	if confIngestSynthetic.FlowLogsPerMin == 0 {
		confIngestSynthetic.FlowLogsPerMin = defaultFlowLogsPerMin
	}
	if confIngestSynthetic.BatchMaxLen == 0 {
		confIngestSynthetic.BatchMaxLen = defaultBatchLen
	}
	log.Debugf("params = %v", confIngestSynthetic)

	return &IngestSynthetic{
		params:           confIngestSynthetic,
		exitChan:         utils.ExitChannel(),
		metricsProcessed: opMetrics.NewCounter(&flowLogsProcessed, params.Name),
	}, nil
}
