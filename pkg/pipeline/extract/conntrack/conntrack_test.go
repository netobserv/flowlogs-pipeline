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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

var opMetrics = operational.NewMetrics(&config.MetricsSettings{})

func buildMockConnTrackConfig(isBidirectional bool, outputRecordType []string,
	updateConnectionInterval, endConnectionTimeout time.Duration) *config.StageParam {
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
						Selector:                 map[string]string{},
						UpdateConnectionInterval: api.Duration{Duration: updateConnectionInterval},
						EndConnectionTimeout:     api.Duration{Duration: endConnectionTimeout},
					},
				},
			}, // end of api.ConnTrack
		}, // end of config.Track
	} // end of config.StageParam
}

func TestTrack(t *testing.T) {
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashId := "705baa5149302fa1"
	hashIdAB := "705baa5149302fa1"
	hashIdBA := "cc40f571f40f3111"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
	flBA4 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 444, 44)
	table := []struct {
		name          string
		conf          *config.StageParam
		inputFlowLogs []config.GenericMap
		expected      []config.GenericMap
	}{
		{
			"bidirectional, output new connection",
			buildMockConnTrackConfig(true, []string{"newConnection"}, updateConnectionInterval, endConnectionTimeout),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashId).get(),
			},
		},
		{
			"bidirectional, output new connection and flow log",
			buildMockConnTrackConfig(true, []string{"newConnection", "flowLog"}, updateConnectionInterval, endConnectionTimeout),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashId).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashId).get(),
				newMockRecordFromFlowLog(flAB2).withHash(hashId).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashId).get(),
				newMockRecordFromFlowLog(flBA4).withHash(hashId).get(),
			},
		},
		{
			"unidirectional, output new connection",
			buildMockConnTrackConfig(false, []string{"newConnection"}, updateConnectionInterval, endConnectionTimeout),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
			},
		},
		{
			"unidirectional, output new connection and flow log",
			buildMockConnTrackConfig(false, []string{"newConnection", "flowLog"}, updateConnectionInterval, endConnectionTimeout),
			[]config.GenericMap{flAB1, flAB2, flBA3, flBA4},
			[]config.GenericMap{
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIdAB).get(),
				newMockRecordFromFlowLog(flAB2).withHash(hashIdAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIdBA).get(),
				newMockRecordFromFlowLog(flBA4).withHash(hashIdBA).get(),
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
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(true, []string{"newConnection", "flowLog", "endConnection"}, updateConnectionInterval, endConnectionTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashId := "705baa5149302fa1"

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
				newMockRecordNewConnAB(ipA, portA, ipB, portB, protocol, 111, 0, 11, 0, 1).withHash(hashId).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashId).get(),
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashId).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashId).get(),
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flBA4).withHash(hashId).get(),
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
				newMockRecordEndConnAB(ipA, portA, ipB, portB, protocol, 333, 777, 33, 77, 4).withHash(hashId).get(),
			},
		},
	}

	for _, tt := range table {
		var prevTime time.Time
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
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
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(false, []string{"newConnection", "flowLog", "endConnection"}, updateConnectionInterval, endConnectionTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashIdAB := "705baa5149302fa1"
	hashIdBA := "cc40f571f40f3111"

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
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIdAB).get(),
			},
		},
		{
			"10s: flow AB and BA",
			startTime.Add(10 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashIdAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIdBA).get(),
			},
		},
		{
			"20s: flow BA",
			startTime.Add(20 * time.Second),
			[]config.GenericMap{flBA4},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flBA4).withHash(hashIdBA).get(),
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
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIdAB).get(),
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
				newMockRecordEndConn(ipB, portB, ipA, portA, protocol, 777, 77, 2).withHash(hashIdBA).get(),
			},
		},
	}

	for _, tt := range table {
		var prevTime time.Time
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
		})
	}
}

// TestUpdateConn_Unidirectional tests that update connection records are outputted correctly and in the right time in
// unidirectional setting.
// The test simulates 2 flow logs from A to B and 2 from B to A in different timestamps.
// Then the test verifies that an update connection record is outputted only after 10 seconds from the last update
// connection report.
func TestUpdateConn_Unidirectional(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(false, []string{"newConnection", "flowLog", "updateConnection", "endConnection"}, updateConnectionInterval, endConnectionTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashIdAB := "705baa5149302fa1"
	hashIdBA := "cc40f571f40f3111"

	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
	flAB2 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 222, 22)
	flBA3 := newMockFlowLog(ipB, portB, ipA, portA, protocol, 333, 33)
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
				newMockRecordNewConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
				newMockRecordFromFlowLog(flAB1).withHash(hashIdAB).get(),
			},
		},
		{
			"5s: flow AB and BA",
			startTime.Add(5 * time.Second),
			[]config.GenericMap{flAB2, flBA3},
			[]config.GenericMap{
				newMockRecordFromFlowLog(flAB2).withHash(hashIdAB).get(),
				newMockRecordNewConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
				newMockRecordFromFlowLog(flBA3).withHash(hashIdBA).get(),
			},
		},
		{
			"9s: no update report",
			startTime.Add(9 * time.Second),
			nil,
			nil,
		},
		{
			"11s: update report AB",
			startTime.Add(11 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIdAB).get(),
			},
		},
		{
			"14s: no update report",
			startTime.Add(14 * time.Second),
			nil,
			nil,
		},
		{
			"16s: update report BA",
			startTime.Add(16 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
			},
		},
		{
			"20s: no update report",
			startTime.Add(20 * time.Second),
			nil,
			nil,
		},
		{
			"22s: update report AB",
			startTime.Add(22 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIdAB).get(),
			},
		},
		{
			"25s: no update report",
			startTime.Add(25 * time.Second),
			nil,
			nil,
		},
		{
			"27s: update report BA",
			startTime.Add(27 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
			},
		},
		{
			"31s: no update report",
			startTime.Add(31 * time.Second),
			nil,
			nil,
		},
		{
			"33s: update report AB",
			startTime.Add(33 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIdAB).get(),
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
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 333, 33, 2).withHash(hashIdAB).get(),
				newMockRecordEndConn(ipB, portB, ipA, portA, protocol, 333, 33, 1).withHash(hashIdBA).get(),
			},
		},
	}

	for _, tt := range table {
		var prevTime time.Time
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
		})
	}
}

// TestIsFirst_LongConnection tests the IsFirst works right in long connections that have multiple updateConnection records.
// In the following test, there should be 2 update connection records and 1 endConnection. Only the first updateConnection record has isFirst set to true.
func TestIsFirst_LongConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(false, []string{"updateConnection", "endConnection"}, updateConnectionInterval, endConnectionTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashIdAB := "705baa5149302fa1"
	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
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
			"9s: no update report",
			startTime.Add(9 * time.Second),
			nil,
			nil,
		},
		{
			"11s: update report AB (with isFirst=true)",
			startTime.Add(11 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).markFirst().get(),
			},
		},
		{
			"20s: no update report",
			startTime.Add(20 * time.Second),
			nil,
			nil,
		},
		{
			"22s: update report AB (with isFirst=false)",
			startTime.Add(22 * time.Second),
			nil,
			[]config.GenericMap{
				newMockRecordUpdateConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
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
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).get(),
			},
		},
	}

	for _, tt := range table {
		var prevTime time.Time
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
		})
	}
}

// TestIsFirst_ShortConnection tests the IsFirst works right in short connections that have only an endConnection record.
// It verifies that this encConnection record has isFirst flag set to true.
func TestIsFirst_ShortConnection(t *testing.T) {
	test.ResetPromRegistry()
	clk := clock.NewMock()
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 5 * time.Second
	conf := buildMockConnTrackConfig(false, []string{"updateConnection", "endConnection"},
		updateConnectionInterval, endConnectionTimeout)
	ct, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocol := 6
	hashIdAB := "705baa5149302fa1"
	flAB1 := newMockFlowLog(ipA, portA, ipB, portB, protocol, 111, 11)
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
				newMockRecordEndConn(ipA, portA, ipB, portB, protocol, 111, 11, 1).withHash(hashIdAB).markFirst().get(),
			},
		},
	}

	for _, tt := range table {
		var prevTime time.Time
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, prevTime, tt.time)
			prevTime = tt.time
			clk.Set(tt.time)
			actual := ct.Extract(tt.inputFlowLogs)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestPrepareUpdateConnectionRecords(t *testing.T) {
	// This test tests prepareUpdateConnectionRecords().
	// It sets the update report interval to 10 seconds and creates 3 records for the first interval and 3 records for the second interval (6 in total).
	// Then, it calls prepareUpdateConnectionRecords() a couple of times in different times.
	// It makes sure that only the right records are returned on each call.
	test.ResetPromRegistry()
	clk := clock.NewMock()
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(false, []string{"updateConnection"}, updateConnectionInterval, endConnectionTimeout)
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
		builder := NewConnBuilder()
		conn := builder.Hash(hash).Build()
		ct.connStore.addConnection(hash.hashTotal, conn)
		conn.setNextUpdateReportTime(r.nextReportTime)
	}
	clk.Set(startTime.Add(interval))
	actual := ct.prepareUpdateConnectionRecords()
	assertHashOrder(t, []uint64{0x01, 0x02, 0x03}, actual)

	clk.Set(startTime.Add(2 * interval))
	actual = ct.prepareUpdateConnectionRecords()
	assertHashOrder(t, []uint64{0x0a, 0x0b, 0x0c}, actual)

	clk.Set(startTime.Add(3 * interval))
	actual = ct.prepareUpdateConnectionRecords()
	assertHashOrder(t, []uint64{0x01, 0x02, 0x03}, actual)
}

// TBD: Test scheduling

func assertHashOrder(t *testing.T, expected []uint64, actualRecords []config.GenericMap) {
	t.Helper()
	var actual []uint64
	for _, r := range actualRecords {
		actual = append(actual, hex2int(r[api.HashIdFieldName].(string)))
	}
	require.Equal(t, expected, actual)
}

func hex2int(hexStr string) uint64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.Replace(hexStr, "0x", "", -1)

	// base 16 for hexadecimal
	result, _ := strconv.ParseUint(cleaned, 16, 64)
	return uint64(result)
}

func TestMaxConnections(t *testing.T) {
	test.ResetPromRegistry()
	maxConnections := 23
	clk := clock.NewMock()
	updateConnectionInterval := 10 * time.Second
	endConnectionTimeout := 30 * time.Second
	conf := buildMockConnTrackConfig(true, []string{"newConnection", "flowLog", "endConnection"}, updateConnectionInterval, endConnectionTimeout)
	conf.Extract.ConnTrack.MaxConnectionsTracked = maxConnections
	extract, err := NewConnectionTrack(opMetrics, *conf, clk)
	require.NoError(t, err)

	ct := extract.(*conntrackImpl)
	require.Equal(t, 0, ct.connStore.mom.Len())

	flowLogs := test.GenerateConnectionEntries(10)
	ct.Extract(flowLogs)
	require.Equal(t, 10, ct.connStore.mom.Len())

	flowLogs = test.GenerateConnectionEntries(20)
	ct.Extract(flowLogs)
	require.Equal(t, 20, ct.connStore.mom.Len())

	flowLogs = test.GenerateConnectionEntries(40)
	ct.Extract(flowLogs)
	require.Equal(t, maxConnections, ct.connStore.mom.Len())
}
