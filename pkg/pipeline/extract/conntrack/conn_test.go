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

package conntrack

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var result bool

var conn = connType{keys: map[string]interface{}{
	"SrcAddr": "10.0.0.1",
	"SrcPort": uint32(30123),
	"DstAddr": "10.0.0.2",
	"DstPort": uint32(80),
	"Proto":   uint32(6),
	"Custom":  time.Time{},
}}

var table = []struct {
	name           string
	selector       map[string]interface{}
	expectedResult bool
}{
	{
		"Proto 1",
		map[string]interface{}{
			"Proto": 6,
		},
		true,
	},
	{
		"Proto 2",
		map[string]interface{}{
			"Proto": 99,
		},
		false,
	},
	{
		"Multiple fields 1",
		map[string]interface{}{
			"Proto":   6,
			"SrcAddr": "10.0.0.1",
			"DstAddr": "10.0.0.2",
			"DstPort": 80,
		},
		true,
	},
	{
		"Multiple fields 2",
		map[string]interface{}{
			"Proto":         6,
			"SrcAddr":       "10.0.0.1",
			"DstAddr":       "10.0.0.2",
			"DstPort":       80,
			"MISSING_FIELD": "",
		},
		false,
	},
	{
		"string match",
		map[string]interface{}{
			"SrcAddr": "10.0.0.1",
		},
		true,
	},
	{
		"string mismatch",
		map[string]interface{}{
			"SrcAddr": "10.0.0.255",
		},
		false,
	},
	{
		"custom match",
		map[string]interface{}{
			"Custom": "0001-01-01 00:00:00 +0000 UTC",
		},
		true,
	},
	{
		"custom mismatch",
		map[string]interface{}{
			"Custom": "0",
		},
		false,
	},
}

func BenchmarkIsMatchSelector(b *testing.B) {
	for _, tt := range table {
		b.Run(tt.name, func(bb *testing.B) {
			var r bool
			bb.StartTimer()
			for i := 0; i < bb.N; i++ {
				r = conn.isMatchSelector(tt.selector)
			}
			bb.StopTimer()
			require.Equal(bb, tt.expectedResult, r)

			// always store the result to a package level variable
			// so the compiler cannot eliminate the Benchmark itself.
			// https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go
			result = r
		})
	}
}

func TestIsMatchSelector(t *testing.T) {
	for _, test := range table {
		t.Run(test.name, func(tt *testing.T) {
			actual := conn.isMatchSelector(test.selector)
			require.Equal(tt, test.expectedResult, actual)
		})
	}
}
