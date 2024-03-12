/*
 * Copyright (C) 2021 IBM, Inc.
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

package aggregate

import (
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func initAggregates(t *testing.T) Aggregates {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: aggregates
      aggregates:
        rules:
          - Name: "Avg by src and dst IP's"
            GroupByKeys:
              - "dstIP"
              - "srcIP"
            OperationType: "avg"
            OperationKey: "value"
`
	v, cfg := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)
	aggregates, err := NewAggregatesFromConfig(cfg.Parameters[0].Extract.Aggregates)
	require.NoError(t, err)

	return aggregates
}

func Test_NewAggregatesFromConfig(t *testing.T) {

	aggregates := initAggregates(t)
	expectedAggregate := GetMockAggregate()

	require.Equal(t, aggregates.Aggregates[0].definition, expectedAggregate.definition)
}

func Test_CleanupExpiredEntriesLoop(t *testing.T) {

	defaultExpiryTime = 4 * time.Second // expiration after 4 seconds
	cleanupLoopTime = 4 * time.Second   // clean up after 4 seconds
	aggregates := initAggregates(t)
	expectedAggregate := GetMockAggregate()
	require.Equal(t, expectedAggregate.definition, aggregates.Aggregates[0].definition)

	entry := test.GetIngestMockEntry(false)
	err := aggregates.Evaluate([]config.GenericMap{entry})
	require.NoError(t, err)
	time.Sleep(2 * time.Second) // still exists after 2 seconds
	require.Equal(t, 1, len(aggregates.Aggregates[0].GetMetrics()))
	time.Sleep(3 * time.Second) // expires after 3 more seconds (5 seconds in total)
	require.Equal(t, 0, len(aggregates.Aggregates[0].GetMetrics()))
}
