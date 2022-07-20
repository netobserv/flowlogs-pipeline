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

package confgen

import (
	"os"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/require"
)

func expectedConfig() *Config {
	return &Config{
		Description: "test description",
		Encode: ConfigEncode{
			Prom: api.PromEncode{
				Port:   7777,
				Prefix: "prefix",
			},
		},
		Ingest: ConfigIngest{
			Collector: api.IngestCollector{
				Port: 8888,
			},
		},
	}
}

const testConfig = `---
## This is the main configuration file for flowlogs-pipeline. It holds
## all parameters needed for the creation of the configuration
##
description:
  test description
ingest:
  collector:
    port: 8888
encode:
  prom:
    port: 7777
    prefix: prefix
`

func Test_parseConfigFile(t *testing.T) {
	filename := "/tmp/config"
	cg := getConfGen()
	err := os.WriteFile(filename, []byte(testConfig), 0644)
	require.Equal(t, err, nil)
	config, err := cg.ParseConfigFile(filename)
	require.NoError(t, err)
	require.Equal(t, config, expectedConfig())
}
