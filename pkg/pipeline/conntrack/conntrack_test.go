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

package conntrack

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func buildConnTrackConfig(isBidirectional bool, outputRecordType []string) *api.ConnTrack {
	splitAB := isBidirectional
	var h api.ConnTrackHash
	if isBidirectional {
		h = api.ConnTrackHash{
			FieldGroupRefs: []string{"protocol"},
			FieldGroupARef: "src",
			FieldGroupBRef: "dst",
		}
	} else {
		h = api.ConnTrackHash{
			FieldGroupRefs: []string{"protocol", "src", "dst"},
		}
	}
	return &api.ConnTrack{
		KeyDefinition: api.KeyDefinition{
			FieldGroups: []api.FieldGroup{
				{
					Name: "src",
					Fields: []string{
						"SrcAddr",
						"SrcPort",
					},
				},
				{
					Name: "dst",
					Fields: []string{
						"DstAddr",
						"DstPort",
					},
				},
				{
					Name: "protocol",
					Fields: []string{
						"Proto",
					},
				},
			},
			Hash: h,
		},
		OutputFields: []api.OutputField{
			{Name: "Bytes", Operation: "sum", SplitAB: splitAB},
			{Name: "Packets", Operation: "sum", SplitAB: splitAB},
			{Name: "numFlowLogs", Operation: "count", SplitAB: false},
		},
		OutputRecordTypes: outputRecordType,
	}
}

func newMockRecordConnAB(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytesAB, bytesBA, packetsAB, packetsBA, numFlowLogs float64) config.GenericMap {
	return config.GenericMap{
		"SrcAddr":     srcIP,
		"SrcPort":     srcPort,
		"DstAddr":     dstIP,
		"DstPort":     dstPort,
		"Proto":       protocol,
		"Bytes_AB":    bytesAB,
		"Bytes_BA":    bytesBA,
		"Packets_AB":  packetsAB,
		"Packets_BA":  packetsBA,
		"numFlowLogs": numFlowLogs,
	}
}

func newMockRecordConn(srcIP string, srcPort int, dstIP string, dstPort int, protocol int, bytes, packets, numFlowLogs float64) config.GenericMap {
	return config.GenericMap{
		"SrcAddr":     srcIP,
		"SrcPort":     srcPort,
		"DstAddr":     dstIP,
		"DstPort":     dstPort,
		"Proto":       protocol,
		"Bytes":       bytes,
		"Packets":     packets,
		"numFlowLogs": numFlowLogs,
	}
}

func TestTrack(t *testing.T) {

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6

	flAB1 := NewFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := NewFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := NewFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
	flBA4 := NewFlowLog(ipB, portB, ipA, portA, protocol, 444, 44)
	table := []struct {
		name          string
		conf          *api.ConnTrack
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"bidirectional, output new connection",
			buildConnTrackConfig(true, []string{"newConnection"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1),
			},
		},
		{
			"bidirectional, output new connection and flow log",
			buildConnTrackConfig(true, []string{"newConnection", "flowLog"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1),
				flAB1,
				flAB2,
				flBA3,
				flBA4,
			},
		},
		{
			"unidirectional, output new connection",
			buildConnTrackConfig(false, []string{"newConnection"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConn(ipA, portA, ipB, portB, protocol, 111, 11, 1),
				newMockRecordConn(ipB, portB, ipA, portA, protocol, 333, 33, 1),
			},
		},
		{
			"unidirectional, output new connection and flow log",
			buildConnTrackConfig(false, []string{"newConnection", "flowLog"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConn(ipA, portA, ipB, portB, protocol, 111, 11, 1),
				flAB1,
				flAB2,
				newMockRecordConn(ipB, portB, ipA, portA, protocol, 333, 33, 1),
				flBA3,
				flBA4,
			},
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			ct, err := NewConnectionTrack(*test.conf)
			require.NoError(t, err)
			actual := ct.Track(test.inputFlowLogs)
			require.Equal(t, test.expected, actual)
		})
	}
}
