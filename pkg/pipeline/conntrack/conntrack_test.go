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
	"time"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func buildMockConnTrackConfig(isBidirectional bool, outputRecordType []string) *api.ConnTrack {
	splitAB := isBidirectional
	var hash api.ConnTrackHash
	if isBidirectional {
		hash = api.ConnTrackHash{
			FieldGroupRefs: []string{"protocol"},
			FieldGroupARef: "src",
			FieldGroupBRef: "dst",
		}
	} else {
		hash = api.ConnTrackHash{
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
			Hash: hash,
		},
		OutputFields: []api.OutputField{
			{Name: "Bytes", Operation: "sum", SplitAB: splitAB},
			{Name: "Packets", Operation: "sum", SplitAB: splitAB},
			{Name: "numFlowLogs", Operation: "count", SplitAB: false},
		},
		OutputRecordTypes:    outputRecordType,
		EndConnectionTimeout: 30 * time.Second,
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

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 444, 44)
	table := []struct {
		name          string
		conf          *api.ConnTrack
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"bidirectional, output new connection",
			buildMockConnTrackConfig(true, []string{"newConnection"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1),
			},
		},
		{
			"bidirectional, output new connection and flow log",
			buildMockConnTrackConfig(true, []string{"newConnection", "flowLog"}),
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
			buildMockConnTrackConfig(false, []string{"newConnection"}),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordConn(ipA, portA, ipB, portB, protocol, 111, 11, 1),
				newMockRecordConn(ipB, portB, ipA, portA, protocol, 333, 33, 1),
			},
		},
		{
			"unidirectional, output new connection and flow log",
			buildMockConnTrackConfig(false, []string{"newConnection", "flowLog"}),
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
			ct, err := NewConnectionTrack(*test.conf, clock.NewMock())
			require.NoError(t, err)
			actual := ct.Track(test.inputFlowLogs)
			require.Equal(t, test.expected, actual)
		})
	}
}

// TestEndConn_Bidirectional tests that end connection records are outputted correctly and in the right time in
// bidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that an end connection record is outputted only after 30 seconds from the last flow log.
func TestEndConn_Bidirectional(t *testing.T) {
	clk := clock.NewMock()
	conf := buildMockConnTrackConfig(true, []string{"newConnection", "flowLog", "endConnection"})
	ct, err := NewConnectionTrack(*conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 444, 44)
	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"start: flow AB",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flAB1},
			[]config.GenericMap{
				newMockRecordConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1),
				flAB1,
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				flAB2,
				flBA3,
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				flBA4,
			},
		},
		{
			"49s: no end conn",
			startTime.Add(49 * time.Second),
			nil,
			nil,
		},
		{
			"51s: end conn BA",
			startTime.Add(51 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordConnAB(ipA, portA, ipB, portB, protocol, 333, 777, 33, 77, 4),
			},
		},
	}

	for _, test := range table {
		var prevTime time.Time
		t.Run(test.name, func(t *testing.T) {
			require.Less(t, prevTime, test.time)
			prevTime = test.time
			clk.Set(test.time)
			actual := ct.Track(test.inputFlowLogs)
			require.Equal(t, test.expected, actual)
		})
	}
}

// TestEndConn_Unidirectional tests that end connection records are outputted correctly and in the right time in
// unidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that an end connection record is outputted only after 30 seconds from the last flow log.
func TestEndConn_Unidirectional(t *testing.T) {
	clk := clock.NewMock()
	conf := buildMockConnTrackConfig(false, []string{"newConnection", "flowLog", "endConnection"})
	ct, err := NewConnectionTrack(*conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 444, 44)
	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"start: flow AB",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flAB1},
			[]config.GenericMap{
				newMockRecordConn(ipA, portA, ipB, portB, protocol, 111, 11, 1),
				flAB1,
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				flAB2,
				newMockRecordConn(ipB, portB, ipA, portA, protocol, 333, 33, 1),
				flBA3,
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				flBA4,
			},
		},
		{
			"39s: no end conn",
			startTime.Add(39 * time.Second),
			nil,
			nil,
		},
		{
			"41s: end conn AB",
			startTime.Add(41 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordConn(ipA, portA, ipB, portB, protocol, 333, 33, 2),
			},
		},
		{
			"49s: no end conn",
			startTime.Add(49 * time.Second),
			nil,
			nil,
		},
		{
			"51s: end conn BA",
			startTime.Add(51 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordConn(ipB, portB, ipA, portA, protocol, 777, 77, 2),
			},
		},
	}

	for _, test := range table {
		var prevTime time.Time
		t.Run(test.name, func(t *testing.T) {
			require.Less(t, prevTime, test.time)
			prevTime = test.time
			clk.Set(test.time)
			actual := ct.Track(test.inputFlowLogs)
			require.Equal(t, test.expected, actual)
		})
	}
}
