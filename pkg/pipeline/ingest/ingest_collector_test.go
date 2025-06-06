/*
 * Copyright (C) 2022 IBM, Inc.
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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeout = 5 * time.Second

func TestIngest(t *testing.T) {
	collectorPort, err := test.UDPPort()
	require.NoError(t, err)
	stage := config.NewCollectorPipeline("ingest-ipfix", api.IngestCollector{
		HostName: "0.0.0.0",
		Port:     collectorPort,
	})
	ic, err := NewIngestCollector(operational.NewMetrics(&config.MetricsSettings{}), stage.GetStageParams()[0])
	require.NoError(t, err)
	forwarded := make(chan config.GenericMap)

	// GIVEN an IPFIX collector Ingester
	go ic.Ingest(forwarded)

	client, err := test.NewIPFIXClient(collectorPort)
	require.NoError(t, err)

	flow := waitForFlow(t, client, forwarded)
	require.NotEmpty(t, flow)
	assert.EqualValues(t, 12345678, flow["TimeFlowStart"])
	assert.EqualValues(t, 12345678, flow["TimeFlowEnd"])
	assert.Equal(t, "1.2.3.4", flow["SrcAddr"])
}

// The IPFIX client might send information before the Ingester is actually listening,
// so we might need to repeat the submission until the ingest starts forwarding logs
func waitForFlow(t *testing.T, client *test.IPFIXClient, forwarded chan config.GenericMap) config.GenericMap {
	var start = time.Now()
	for {
		if client.SendTemplate() == nil &&
			client.SendFlow(12345678, "1.2.3.4") == nil {
			select {
			case received := <-forwarded:
				return received
			default:
				// nothing yet received
			}
		}
		if time.Since(start) > timeout {
			require.Fail(t, "error waiting for ingester to forward received data")
		}
		time.After(50 * time.Millisecond)
	}
}
