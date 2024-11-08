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
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

var opMetrics = operational.NewMetrics(&config.MetricsSettings{})

func buildMockConnTrackConfig(isBidirectional bool, outputRecordType []api.ConnTrackOutputRecordTypeEnum,
	heartbeatInterval, endConnectionTimeout time.Duration, terminatingTimeout time.Duration) *config.StageParam {
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
	return &config.StageParam{
		Extract: &config.Extract{
			Type: api.ConnTrackType,
			ConnTrack: &api.ConnTrack{
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
				OutputRecordTypes: outputRecordType,
				Scheduling: []api.ConnTrackSchedulingGroup{
					{
						Selector:             map[string]interface{}{},
						HeartbeatInterval:    api.Duration{Duration: heartbeatInterval},
						EndConnectionTimeout: api.Duration{Duration: endConnectionTimeout},
						TerminatingTimeout:   api.Duration{Duration: terminatingTimeout},
					},
				},
			}, // end of api.ConnTrack
		}, // end of config.Track
	} // end of config.StageParam
}

func TestTrack(t *testing.T) {
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashID := "705baa5149302fa1"
	hashIDAB := "705baa5149302fa1"
	hashIDBA := "cc40f571f40f3111"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
	// duplicates should be ignored
	flAB1Duplicated := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, true)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 222, 22, false)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 333, 33, false)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 444, 44, false)
	table := []struct {
		name          string
		conf          *config.StageParam
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"duplicates, doesn't output connection events",
			buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "heartbeat", "endConnection"},
				heartbeatInterval, endConnectionTimeout, terminatingTimeout),
			[]config.GenericMap{flAB1Duplicated},
			[]config.GenericMap(nil),
		},
		{
			"bidirectional, output new connection",
			buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection"},
				heartbeatInterval, endConnectionTimeout, terminatingTimeout),
			[]config.GenericMap{flAB1, flAB1Duplicated, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashID).get(),
			},
		},
		{
			"bidirectional, output new connection and flow log",
			buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog"},
				heartbeatInterval, endConnectionTimeout, terminatingTimeout),
			[]config.GenericMap{flAB1, flAB1Duplicated, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashID).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashID).get(),
				newMockRecordFromFlowLog(flAB1Duplicated).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB2).withHash(hashID).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashID).get(),
				newMockRecordFromFlowLog(flBA4).withHash(hashID).get(),
			},
		},
		{
			"unidirectional, output new connection",
			buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"newConnection"},
				heartbeatInterval, endConnectionTimeout, terminatingTimeout),
			[]config.GenericMap{flAB1, flAB1Duplicated, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
			},
		},
		{
			"unidirectional, output new connection and flow log",
			buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog"},
				heartbeatInterval, endConnectionTimeout, terminatingTimeout),
			[]config.GenericMap{flAB1, flAB1Duplicated, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB1Duplicated).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB2).withHash(hashIDAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIDBA).get(),
				newMockRecordFromFlowLog(flBA4).withHash(hashIDBA).get(),
			},
		},
	}

	for _, testt := range table {
		t.Run(testt.name, func(t *testing.T) {
			test.ResetPromRegistry()
			ct, err := NewConnectionTrack(opMetrics, *testt.conf, clock.NewMock())
			require.NoError(t, err)
			actual := ct.Extract(testt.inputFlowLogs)
			require.Equal(t, testt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

// TestEndConn_Bidirectional tests that end connection records are outputted correctly and in the right time in
// bidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that an end connection record is outputted only after 30 seconds from the last flow log.
func TestEndConn_Bidirectional(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashID := "705baa5149302fa1"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 222, 22, false)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 333, 33, false)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 444, 44, false)
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
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashID).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashID).get(),
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashID).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashID).get(),
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flBA4).withHash(hashID).get(),
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
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocol, 333, 777, 33, 77, 4).withHash(hashID).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

// TestEndConn_Unidirectional tests that end connection records are outputted correctly and in the right time in
// unidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that an end connection record is outputted only after 30 seconds from the last flow log.
func TestEndConn_Unidirectional(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashIDAB := "705baa5149302fa1"
	hashIDBA := "cc40f571f40f3111"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 222, 22, false)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 333, 33, false)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 444, 44, false)
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
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIDAB).get(),
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashIDAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIDBA).get(),
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flBA4).withHash(hashIDBA).get(),
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
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIDAB).get(),
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
				newMockRecordEndConn(ipB, portB, ipA, portA, protocol, 777, 77, 2).withHash(hashIDBA).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

// TestHeartbeat_Unidirectional tests that heartbeat records are outputted correctly and in the right time in
// unidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that a heartbeat record is outputted only after 10 seconds from the last heartbeat.
func TestHeartbeat_Unidirectional(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog", "heartbeat", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashIDAB := "705baa5149302fa1"
	hashIDBA := "cc40f571f40f3111"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 222, 22, false)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, flowDir, 333, 33, false)
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
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIDAB).get(),
			},
		},
		{
			"5s: flow AB and BA",
			startTime.Add(5 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashIDAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIDBA).get(),
			},
		},
		{
			"9s: no heartbeat",
			startTime.Add(9 * time.Second),
			nil,
			nil,
		},
		{
			"11s: heartbeat AB",
			startTime.Add(11 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIDAB).get(),
			},
		},
		{
			"14s: no heartbeat",
			startTime.Add(14 * time.Second),
			nil,
			nil,
		},
		{
			"16s: heartbeat BA",
			startTime.Add(16 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
			},
		},
		{
			"20s: no heartbeat",
			startTime.Add(20 * time.Second),
			nil,
			nil,
		},
		{
			"22s: heartbeat AB",
			startTime.Add(22 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIDAB).get(),
			},
		},
		{
			"25s: no heartbeat",
			startTime.Add(25 * time.Second),
			nil,
			nil,
		},
		{
			"27s: heartbeat BA",
			startTime.Add(27 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
			},
		},
		{
			"31s: no heartbeat",
			startTime.Add(31 * time.Second),
			nil,
			nil,
		},
		{
			"33s: heartbeat AB",
			startTime.Add(33 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIDAB).get(),
			},
		},
		{
			"34s: no end conn",
			startTime.Add(34 * time.Second),
			nil,
			nil,
		},
		{
			"36s: end conn AB and BA",
			startTime.Add(36 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIDAB).get(),
				newMockRecordEndConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIDBA).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

// TestIsFirst_LongConnection tests the IsFirst works right in long connections that have multiple heartbeat records.
// In the following test, there should be 2 heartbeat records and 1 endConnection. Only the first heartbeat record has isFirst set to true.
func TestIsFirst_LongConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"heartbeat", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashIDAB := "705baa5149302fa1"
	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
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
			nil,
		},
		{
			"9s: no heartbeat",
			startTime.Add(9 * time.Second),
			nil,
			nil,
		},
		{
			"11s: heartbeat AB (with isFirst=true)",
			startTime.Add(11 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).markFirst().get(),
			},
		},
		{
			"20s: no heartbeat",
			startTime.Add(20 * time.Second),
			nil,
			nil,
		},
		{
			"22s: heartbeat AB (with isFirst=false)",
			startTime.Add(22 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeat(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
			},
		},
		{
			"29s: no end conn",
			startTime.Add(29 * time.Second),
			nil,
			nil,
		},
		{
			"31s: end conn AB",
			startTime.Add(31 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

// TestIsFirst_ShortConnection tests the IsFirst works right in short connections that have only an endConnection record.
// It verifies that this encConnection record has isFirst flag set to true.
func TestIsFirst_ShortConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 5 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"heartbeat", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	flowDir := 0
	hashIDAB := "705baa5149302fa1"
	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, flowDir, 111, 11, false)
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
			nil,
		},
		{
			"4s: no end conn",
			startTime.Add(4 * time.Second),
			nil,
			nil,
		},
		{
			"6s: end conn AB (with isFirst=true)",
			startTime.Add(6 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIDAB).markFirst().get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
}

func TestPrepareUpdateConnectionRecords(t *testing.T) {
	// This test tests prepareHeartbeatRecords().
	// It sets the heartbeat interval to 10 seconds and creates 3 records for the first interval and 3 records for the second interval (6 in total).
	// Then, it calls prepareHeartbeatRecords() a couple of times in different times.
	// It makes sure that only the right records are returned on each call.
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(false, []api.ConnTrackOutputRecordTypeEnum{"heartbeat"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	interval := 10 * time.Second
	extract, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)
	ct := extract.(*conntrackImpl)
	startTime := clk.Now()

	reportTimes := []struct {
		nextReportTime time.Time
		hash           uint64
	}{
		{startTime.Add(1 * time.Second), 0x01},
		{startTime.Add(2 * time.Second), 0x02},
		{startTime.Add(3 * time.Second), 0x03},
		{startTime.Add(interval + 1*time.Second), 0x0a},
		{startTime.Add(interval + 2*time.Second), 0x0b},
		{startTime.Add(interval + 3*time.Second), 0x0c},
	}
	for _, r := range reportTimes {
		hash := totalHashType{hashTotal: r.hash}
		builder := newConnBuilder(nil)
		conn := builder.Hash(hash).Build()
		ct.connStore.addConnection(hash.hashTotal, conn)
		conn.setNextHeartbeatTime(r.nextReportTime)
	}
	clk.Set(startTime.Add(interval))
	actual := ct.prepareHeartbeatRecords()
	assertHashOrder(t, []uint64{0x01, 0x02, 0x03}, actual)

	clk.Set(startTime.Add(2 * interval))
	actual = ct.prepareHeartbeatRecords()
	assertHashOrder(t, []uint64{0x0a, 0x0b, 0x0c}, actual)

	clk.Set(startTime.Add(3 * interval))
	actual = ct.prepareHeartbeatRecords()
	assertHashOrder(t, []uint64{0x01, 0x02, 0x03}, actual)
}

// TestScheduling tests scheduling groups. It configures 2 scheduling groups:
//  1. UDP connections
//  2. default group (matches TCP connections among other things)
//
// Then, it creates 4 flow logs: 2 that belong to an UDP connection and 2 that belong to a TCP connection.
// The test verifies that heartbeat and endConnection records are emitted at the right timestamps for each
// connection according to its scheduling group.
// The timeline events of the test is as follows ("I" and "O" indicates input and output):
// 0s:  I flow 			TCP
// 0s:  I flow 			UDP
// 10s: I flow 			TCP
// 15s: I flow			UDP
// 20s: O heartbeat		TCP
// 25s: O endConn 		TCP
// 30s: O heartbeat		UDP
// 35s: O endConn 		UDP
func TestScheduling(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	defaultHeartbeatInterval := 20 * time.Second
	defaultEndConnectionTimeout := 15 * time.Second
	defaultTerminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"heartbeat", "endConnection"},
		defaultHeartbeatInterval, defaultEndConnectionTimeout, defaultTerminatingTimeout)
	// Insert a scheduling group before the default group.
	// https://github.com/golang/go/wiki/SliceTricks#push-frontunshift
	conf.Extract.ConnTrack.Scheduling = append(
		[]api.ConnTrackSchedulingGroup{
			{
				Selector:             map[string]interface{}{"Proto": 17}, // UDP
				HeartbeatInterval:    api.Duration{Duration: 30 * time.Second},
				EndConnectionTimeout: api.Duration{Duration: 20 * time.Second},
				TerminatingTimeout:   api.Duration{Duration: 10 * time.Second},
			},
		},
		conf.Extract.ConnTrack.Scheduling...)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocolTCP := 6
	protocolUDP := 17
	flowDir := 0
	hashIDTCP := "705baa5149302fa1"
	hashIDUDP := "70fb44d4ad00d2e2"
	flTCP1 := newMockFlowLog(ipA, portA, ipB, portB, protocolTCP, flowDir, 111, 11, false)
	flTCP2 := newMockFlowLog(ipB, portB, ipA, portA, protocolTCP, flowDir, 222, 22, false)
	flUDP1 := newMockFlowLog(ipA, portA, ipB, portB, protocolUDP, flowDir, 333, 33, false)
	flUDP2 := newMockFlowLog(ipB, portB, ipA, portA, protocolUDP, flowDir, 444, 44, false)
	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"start: flow TCP, flow UDP",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flTCP1, flUDP1},
			nil,
		},
		{
			"10s: flow TCP",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flTCP2},
			nil,
		},
		{
			"15s: flow UDP",
			startTime.Add(15 * time.Second),
			[]config.GenericMap{flUDP2},
			nil,
		},
		{
			"19s: no heartbeat",
			startTime.Add(19 * time.Second),
			nil,
			nil,
		},
		{
			"21s: heartbeat TCP conn",
			startTime.Add(21 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeatAB(ipA, portA, ipB, portB, protocolTCP, 111, 222, 11, 22, 2).withHash(hashIDTCP).markFirst().get(),
			},
		},
		{
			"24s: no end conn",
			startTime.Add(24 * time.Second),
			nil,
			nil,
		},
		{
			"26s: end conn TCP",
			startTime.Add(26 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocolTCP, 111, 222, 11, 22, 2).withHash(hashIDTCP).get(),
			},
		},
		{
			"29s: no heartbeat",
			startTime.Add(29 * time.Second),
			nil,
			nil,
		},
		{
			"31s: heartbeat UDP conn",
			startTime.Add(31 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordHeartbeatAB(ipA, portA, ipB, portB, protocolUDP, 333, 444, 33, 44, 2).withHash(hashIDUDP).markFirst().get(),
			},
		},
		{
			"34s: no end conn",
			startTime.Add(34 * time.Second),
			nil,
			nil,
		},
		{
			"36s: end conn UDP",
			startTime.Add(36 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocolUDP, 333, 444, 33, 44, 2).withHash(hashIDUDP).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
	exposed := test.ReadExposedMetrics(t, prometheus.DefaultGatherer)
	require.Contains(t, exposed, `conntrack_end_connections{group="0: Proto=17, ",reason="timeout"} 1`)
}

func assertStoreConsistency(t *testing.T, extractor extract.Extractor) {
	store := extractor.(*conntrackImpl).connStore
	hashLen := len(store.hashID2groupIdx)
	groupsLenSlice := make([]int, 0)
	sumGroupsLen := 0
	for _, g := range store.groups {
		sumGroupsLen += g.terminatingMom.Len()
		sumGroupsLen += g.activeMom.Len()
		groupsLenSlice = append(groupsLenSlice, g.terminatingMom.Len()+g.activeMom.Len())
	}
	require.Equal(t, hashLen, sumGroupsLen, "hashLen(=%v) != sum(%v)", hashLen, groupsLenSlice)
}

func assertHashOrder(t *testing.T, expected []uint64, actualRecords []config.GenericMap) {
	t.Helper()
	var actual []uint64
	for _, r := range actualRecords {
		actual = append(actual, hex2int(r[api.HashIDFieldName].(string)))
	}
	require.Equal(t, expected, actual)
}

func hex2int(hexStr string) uint64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.ReplaceAll(hexStr, "0x", "")

	// base 16 for hexadecimal
	result, _ := strconv.ParseUint(cleaned, 16, 64)
	return uint64(result)
}

func TestMaxConnections(t *testing.T) {
	test.ResetPromRegistry()
	maxConnections := 23
	clk := clock.NewMock()
	heartbeatInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "flowLog", "endConnection"},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	conf.Extract.ConnTrack.MaxConnectionsTracked = maxConnections
	extract, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ct := extract.(*conntrackImpl)
	require.Equal(t, 0, ct.connStore.len())

	flowLogs := utils.GenerateConnectionFlowEntries(10)
	ct.Extract(flowLogs)
	require.Equal(t, 10, ct.connStore.len())

	flowLogs = utils.GenerateConnectionFlowEntries(20)
	ct.Extract(flowLogs)
	require.Equal(t, 20, ct.connStore.len())

	flowLogs = utils.GenerateConnectionFlowEntries(40)
	ct.Extract(flowLogs)
	require.Equal(t, maxConnections, ct.connStore.len())
}

func TestGenerateConnectionFlowEntries(t *testing.T) {
	n := 100000
	flowLogs := utils.GenerateConnectionFlowEntries(n)
	require.Equal(t, n, len(flowLogs))
}

func TestIsLastFlowLogOfConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	heartbeatInterval := 30 * time.Second
	endConnectionTimeout := 10 * time.Second
	terminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{},
		heartbeatInterval, endConnectionTimeout, terminatingTimeout)
	tcpFlagsFieldName := "TCPFlags"
	conf.Extract.ConnTrack.TCPFlags = api.ConnTrackTCPFlags{
		FieldName:           tcpFlagsFieldName,
		DetectEndConnection: true,
	}
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)
	table := []struct {
		name         string
		inputFlowLog config.GenericMap
		flag         uint32
		expected     bool
	}{
		{
			"Happy path",
			config.GenericMap{tcpFlagsFieldName: uint32(FINFlag)},
			FINFlag,
			true,
		},
		{
			"Multiple flags 1",
			config.GenericMap{tcpFlagsFieldName: uint32(FINFlag | SYNACKFlag)},
			FINFlag,
			true,
		},
		{
			"Multiple flags 2",
			config.GenericMap{tcpFlagsFieldName: uint32(FINFlag | SYNACKFlag)},
			SYNACKFlag,
			true,
		},
		{
			"Convert from string",
			config.GenericMap{tcpFlagsFieldName: fmt.Sprint(FINFlag)},
			FINFlag,
			true,
		},
		{
			"Cannot parse value",
			config.GenericMap{tcpFlagsFieldName: ""},
			FINFlag,
			false,
		},
		{
			"Other flag than FIN",
			config.GenericMap{tcpFlagsFieldName: FINACKFlag},
			FINFlag,
			false,
		},
		{
			"Missing TCPFlags field",
			config.GenericMap{"": ""},
			FINFlag,
			false,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			actual := ct.(*conntrackImpl).containsTCPFlag(tt.inputFlowLog, tt.flag)
			require.Equal(t, tt.expected, actual)
		})
	}

}

func TestDetectEndConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	defaultUpdateConnectionInterval := 30 * time.Second
	defaultEndConnectionTimeout := 10 * time.Second
	defaultTerminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "endConnection"},
		defaultUpdateConnectionInterval, defaultEndConnectionTimeout, defaultTerminatingTimeout)
	tcpFlagsFieldName := "TCPFlags"
	conf.Extract.ConnTrack.TCPFlags = api.ConnTrackTCPFlags{
		FieldName:           tcpFlagsFieldName,
		DetectEndConnection: true,
	}
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocolTCP := 6
	flowDir := 0
	hashIDTCP := "705baa5149302fa1"
	flTCP1 := newMockFlowLog(ipA, portA, ipB, portB, protocolTCP, flowDir, 111, 11, false)
	flTCP2 := newMockFlowLog(ipB, portB, ipA, portA, protocolTCP, flowDir, 222, 22, false)
	// duplicates should be ignored
	flTCP2Duplicated := newMockFlowLog(ipB, portB, ipA, portA, protocolTCP, flowDir, 222, 22, true)

	flTCP2[tcpFlagsFieldName] = FINFlag

	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"start: new connection",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flTCP1},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocolTCP, 111, 0, 11, 0, 1).withHash(hashIDTCP).markFirst().get(),
			},
		},
		{
			"5s: end connection",
			startTime.Add(5 * time.Second),
			[]config.GenericMap{flTCP2},
			[]config.GenericMap(nil),
		},
		{
			"6s: end connection duplicated",
			startTime.Add(6 * time.Second),
			[]config.GenericMap{flTCP2Duplicated},
			[]config.GenericMap(nil),
		},
		{
			"20s: end connection without duplicated",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{},
			[]config.GenericMap{
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocolTCP, 111, 222, 11, 22, 2).withHash(hashIDTCP).get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
	exposed := test.ReadExposedMetrics(t, prometheus.DefaultGatherer)
	require.Contains(t, exposed, `conntrack_tcp_flags{action="detectEndConnection"} 1`)
	require.Contains(t, exposed, `conntrack_input_records{classification="duplicate"} 1`)
}

func TestSwapAB(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	defaultUpdateConnectionInterval := 30 * time.Second
	defaultEndConnectionTimeout := 10 * time.Second
	defaultTerminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "endConnection"},
		defaultUpdateConnectionInterval, defaultEndConnectionTimeout, defaultTerminatingTimeout)
	tcpFlagsFieldName := "TCPFlags"
	conf.Extract.ConnTrack.TCPFlags = api.ConnTrackTCPFlags{
		FieldName: tcpFlagsFieldName,
		SwapAB:    true,
	}
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocolTCP := 6
	flowDir := 0
	hashIDTCP := "705baa5149302fa1"
	flTCP1 := newMockFlowLog(ipB, portB, ipA, portA, protocolTCP, flowDir, 111, 11, false)
	flTCP1[tcpFlagsFieldName] = SYNACKFlag

	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"new connection (A and B are swapped)",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flTCP1},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocolTCP, 0, 111, 0, 11, 1).withHash(hashIDTCP).markFirst().get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
	exposed := test.ReadExposedMetrics(t, prometheus.DefaultGatherer)
	require.Contains(t, exposed, `conntrack_tcp_flags{action="swapAB"} 1`)
}

func TestExpiringConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	defaultUpdateConnectionInterval := 30 * time.Second
	defaultEndConnectionTimeout := 10 * time.Second
	defaultTerminatingTimeout := 5 * time.Second

	conf := buildMockConnTrackConfig(true, []api.ConnTrackOutputRecordTypeEnum{"newConnection", "endConnection"},
		defaultUpdateConnectionInterval, defaultEndConnectionTimeout, defaultTerminatingTimeout)
	tcpFlagsFieldName := "TCPFlags"
	conf.Extract.ConnTrack.TCPFlags = api.ConnTrackTCPFlags{
		FieldName:           tcpFlagsFieldName,
		DetectEndConnection: true,
	}
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocolTCP := 6
	flowDir := 0
	hashIDTCP := "705baa5149302fa1"
	flTCP1 := newMockFlowLog(ipA, portA, ipB, portB, protocolTCP, flowDir, 111, 11, false)
	flTCP2 := newMockFlowLog(ipB, portB, ipA, portA, protocolTCP, flowDir, 222, 22, false)
	flTCP2[tcpFlagsFieldName] = FINFlag
	flTCP3 := newMockFlowLog(ipA, portA, ipB, portB, protocolTCP, flowDir, 333, 33, false)
	flTCP4 := newMockFlowLog(ipA, portA, ipB, portB, protocolTCP, flowDir, 555, 55, false)

	startTime := clk.Now()
	table := []struct {
		name          string
		time          time.Time
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"start: new connection",
			startTime.Add(0 * time.Second),
			[]config.GenericMap{flTCP1},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocolTCP, 111, 0, 11, 0, 1).withHash(hashIDTCP).markFirst().get(),
			},
		},
		{
			"5s: FIN flow log to end the connection",
			startTime.Add(5 * time.Second),
			[]config.GenericMap{flTCP2},
			[]config.GenericMap(nil),
		},
		{
			"10s: flow log after FIN is still part of the connection",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flTCP3},
			[]config.GenericMap(nil),
		},
		{
			"16s: end connection. The after-FIN-flow-log doesn't extend the connection's life",
			startTime.Add(16 * time.Second),
			[]config.GenericMap{},
			[]config.GenericMap{
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocolTCP, 444, 222, 44, 22, 3).withHash(hashIDTCP).get(),
			},
		},
		{
			"17s: another flow log will create a new connection",
			startTime.Add(17 * time.Second),
			[]config.GenericMap{flTCP4},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocolTCP, 555, 0, 55, 0, 1).withHash(hashIDTCP).markFirst().get(),
			},
		},
	}

	var prevTime time.Time
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
			assertStoreConsistency(t, ct)
		})
	}
	exposed := test.ReadExposedMetrics(t, prometheus.DefaultGatherer)
	require.Contains(t, exposed, `conntrack_end_connections{group="0: DEFAULT",reason="FIN_flag"} 1`)
}
