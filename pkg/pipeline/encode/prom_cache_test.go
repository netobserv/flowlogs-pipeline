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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
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
       expiryTime: 1s
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
       expiryTime: 1s
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
       expiryTime: 1s
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
