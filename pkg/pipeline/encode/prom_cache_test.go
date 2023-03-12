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
package encode

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

var (
	yamlConfig1 = `
pipeline:
 - name: encode
parameters:
 - name: encode_prom
   encode:
     type: prom
     prom:
       prefix: test_
       expiryTime: 1
       maxMetrics: 30
       metrics:
         - name: bytes_count
           type: counter
           valueKey: Bytes
           labels:
             - SrcAddr
`

	yamlConfig2 = `
pipeline:
 - name: encode
parameters:
 - name: encode_prom
   encode:
     type: prom
     prom:
       prefix: test_
       expiryTime: 1
       maxMetrics: 30
       metrics:
         - name: bytes_count
           type: counter
           valueKey: Bytes
           labels:
             - SrcAddr
         - name: packets_count
           type: counter
           valueKey: Packets
           labels:
             - SrcAddr
`

	yamlConfig3 = `
pipeline:
 - name: encode
parameters:
 - name: encode_prom
   encode:
     type: prom
     prom:
       prefix: test_
       expiryTime: 1
       metrics:
         - name: bytes_count
           type: counter
           valueKey: Bytes
           labels:
             - SrcAddr
         - name: packets_count
           type: counter
           valueKey: Packets
           labels:
             - SrcAddr
`
)

func encodeEntries(promEncode *EncodeProm, entries []config.GenericMap) {
	for _, entry := range entries {
		promEncode.Encode(entry)
	}
}

// Test_Prom_Cache tests the integration between encode_prom and timebased_cache.
// Set a cache size, create many prom metrics, and verify that they interact properly.
func Test_Prom_Cache1(t *testing.T) {
	var entries []config.GenericMap

	v, cfg := test.InitConfig(t, yamlConfig1)
	require.NotNil(t, v)

	promEncode, err := initProm(cfg.Parameters[0].Encode.Prom)
	require.NoError(t, err)

	entries = utils.GenerateConnectionFlowEntries(10)
	require.Equal(t, 10, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 10, promEncode.mCache.GetCacheLen())

	entries = utils.GenerateConnectionFlowEntries(40)
	require.Equal(t, 40, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 30, promEncode.mCache.GetCacheLen())
}

func Test_Prom_Cache2(t *testing.T) {
	var entries []config.GenericMap

	v, cfg := test.InitConfig(t, yamlConfig2)
	require.NotNil(t, v)

	promEncode, err := initProm(cfg.Parameters[0].Encode.Prom)
	require.NoError(t, err)

	entries = utils.GenerateConnectionFlowEntries(10)
	require.Equal(t, 10, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 20, promEncode.mCache.GetCacheLen())

	entries = utils.GenerateConnectionFlowEntries(40)
	require.Equal(t, 40, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 30, promEncode.mCache.GetCacheLen())
}

func Test_Prom_Cache3(t *testing.T) {
	var entries []config.GenericMap

	v, cfg := test.InitConfig(t, yamlConfig3)
	require.NotNil(t, v)

	promEncode, err := initProm(cfg.Parameters[0].Encode.Prom)
	require.NoError(t, err)

	entries = utils.GenerateConnectionFlowEntries(10)
	require.Equal(t, 10, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 20, promEncode.mCache.GetCacheLen())

	entries = utils.GenerateConnectionFlowEntries(40)
	require.Equal(t, 40, len(entries))
	encodeEntries(promEncode, entries)
	require.Equal(t, 80, promEncode.mCache.GetCacheLen())
}
