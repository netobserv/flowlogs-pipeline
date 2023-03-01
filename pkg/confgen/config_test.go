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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func expectedConfig() *Config {
	return &Config{
		Description: "test description",
		Encode: config.Encode{
			Prom: &api.PromEncode{
				Prefix: "prefix",
			},
		},
		Ingest: config.Ingest{
			Collector: &api.IngestCollector{
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
    prefix: prefix
`

func Test_parseConfigFile(t *testing.T) {
	file, err := os.CreateTemp("", "config.yaml")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	cg := NewConfGen(&Options{})
	_, err = file.Write([]byte(testConfig))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	config, err := cg.ParseConfigFile(file.Name())
	require.NoError(t, err)
	require.Equal(t, config, expectedConfig())
}
