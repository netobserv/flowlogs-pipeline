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
	metricsProcessed = operational.DefineMetric(
		"ingest_synthetic_flows_processed",
		"Number of metrics processed",
		operational.TypeCounter,
		"stage",
	)
)

// IngestSynthetic Ingest generates flow logs according to provided parameters
func (ingestS *IngestSynthetic) Ingest(out chan<- config.GenericMap) {
	log.Debugf("entering IngestSynthetic Ingest, params = %v", ingestS.params)
	flowLogs := utils.GenerateConnectionFlowEntries(ingestS.params.Connections)
	nLogs := len(flowLogs)
	next := 0

	// compute time interval between batches
	ticker := time.NewTicker(time.Duration(int(time.Minute*time.Duration(ingestS.params.BatchMaxLen)) / ingestS.params.FlowLogsPerMin))

	// loop forever
	for {
		select {
		case <-ingestS.exitChan:
			log.Debugf("exiting IngestSynthetic because of signal")
			return
		case <-ticker.C:
			flowsLeft := ingestS.params.BatchMaxLen
			log.Debugf("flowsLeft = %d", flowsLeft)
			batchLen := flowsLeft
			for flowsLeft > 0 {
				remainder := nLogs - next
				if batchLen > remainder {
					batchLen = remainder
				}
				log.Debugf("flowsLeft = %d, remainder = %d, batchLen = %d", flowsLeft, remainder, batchLen)
				batch := flowLogs[next : next+batchLen]
				ingestS.sendBatch(batch, out)
				ingestS.metricsProcessed.Add(float64(batchLen))
				flowsLeft -= batchLen
				next += batchLen
				if batchLen == remainder {
					next = 0
					batchLen = flowsLeft
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
	jsonIngestSynthetic := api.IngestSynthetic{}
	if params.Ingest != nil || params.Ingest.Synthetic != nil {
		jsonIngestSynthetic = *params.Ingest.Synthetic
	}
	if jsonIngestSynthetic.Connections == 0 {
		jsonIngestSynthetic.Connections = defaultConnections
	}
	if jsonIngestSynthetic.FlowLogsPerMin == 0 {
		jsonIngestSynthetic.FlowLogsPerMin = defaultFlowLogsPerMin
	}
	if jsonIngestSynthetic.BatchMaxLen == 0 {
		jsonIngestSynthetic.BatchMaxLen = defaultBatchLen
	}
	log.Debugf("params = %v", jsonIngestSynthetic)

	return &IngestSynthetic{
		params:           jsonIngestSynthetic,
		exitChan:         utils.ExitChannel(),
		metricsProcessed: opMetrics.NewCounter(&metricsProcessed, params.Name),
	}, nil
}
