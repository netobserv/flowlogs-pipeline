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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_NewAggregatesFromConfig(t *testing.T) {
	var yamlConfig = `
log-level: debug
pipeline:
  - name: extract1
parameters:
  - name: extract1
    extract:
      type: aggregates
      aggregates:
        - Name: "Avg by src and dst IP's"
          By:
            - "dstIP"
            - "srcIP"
          Operation: "avg"
          RecordKey: "value"
`
	v := test.InitConfig(t, yamlConfig)
	require.NotNil(t, v)

	expectedAggregate := GetMockAggregate()
	aggregates, err := NewAggregatesFromConfig(config.Parameters[0].Extract.Aggregates)

	require.NoError(t, err)
	require.Equal(t, aggregates[0].Definition, expectedAggregate.Definition)
}
