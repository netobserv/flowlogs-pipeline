/*
 * Copyright (C) 2023 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestIngestSynthetic(t *testing.T) {
	// check default values
	params := config.StageParam{
		Ingest: &config.Ingest{
			Type:      "synthetic",
			Synthetic: &api.IngestSynthetic{},
		},
	}
	ingest, err := NewIngestSynthetic(params)
	syn := ingest.(*IngestSynthetic)
	require.NoError(t, err)
	require.Equal(t, defaultBatchLen, syn.params.BatchMaxLen)
	require.Equal(t, defaultConnections, syn.params.Connections)
	require.Equal(t, defaultFlowLogsPerMin, syn.params.FlowLogsPerMin)

	batchMaxLen := 3
	connections := 20
	flowLogsPerMin := 1000
	synthetic := api.IngestSynthetic{
		BatchMaxLen:    batchMaxLen,
		Connections:    connections,
		FlowLogsPerMin: flowLogsPerMin,
	}
	params = config.StageParam{
		Ingest: &config.Ingest{
			Type:      "synthetic",
			Synthetic: &synthetic,
		},
	}
	ingest, err = NewIngestSynthetic(params)
	syn = ingest.(*IngestSynthetic)
	require.NoError(t, err)
	require.Equal(t, batchMaxLen, syn.params.BatchMaxLen)
	require.Equal(t, connections, syn.params.Connections)
	require.Equal(t, flowLogsPerMin, syn.params.FlowLogsPerMin)

	// run the Ingest method in a separate thread
	ingestOutput := make(chan config.GenericMap)
	go syn.Ingest(ingestOutput)

	type connection struct {
		srcAddr  string
		dstAddr  string
		srcPort  int
		dstPort  int
		protocol int
	}

	// Start collecting flows from the ingester and ensure we have the specified number of distinct connections
	connectionMap := make(map[connection]int)
	for i := 0; i < (3 * connections); i++ {
		flowEntry := <-ingestOutput
		conn := connection{
			srcAddr:  flowEntry["SrcAddr"].(string),
			dstAddr:  flowEntry["DstAddr"].(string),
			srcPort:  flowEntry["SrcPort"].(int),
			dstPort:  flowEntry["DstPort"].(int),
			protocol: flowEntry["Proto"].(int),
		}
		connectionMap[conn]++
	}
	require.Equal(t, connections, len(connectionMap))
}
