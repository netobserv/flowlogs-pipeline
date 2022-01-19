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
	"bytes"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.ibm.com/MCNM/observability/flowlogs2metrics/pkg/config"
	"testing"
)

func Test_NewAggregatesFromConfig(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  ingest:
    type: collector
  decode:
    type: json
  extract:
    type: aggregates
    aggregates:
    - Name: "Avg by src and dst IP's"
      By:
      - "dstIP"
      - "srcIP"
      Operation: "avg"
      RecordKey: "value"
  encode:
    type: prom
  write:
    type: stdout
`)
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader(yamlConfig)
	err := v.ReadConfig(r)
	require.Equal(t, err, nil)
	val := v.Get("pipeline.extract.aggregates")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)
	config.Opt.PipeLine.Extract.Aggregates = string(b)

	expectedAggregate := GetMockAggregate()
	aggregates, err := NewAggregatesFromConfig()

	require.Equal(t, err, nil)
	require.Equal(t, aggregates[0].Definition, expectedAggregate.Definition)
}
