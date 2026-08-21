/*
 * Copyright (C) 2026 NetObserv Authors.
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

package flowbuffer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func flow(ts int64, ns string) config.GenericMap {
	return config.GenericMap{
		"TimeFlowStartMs":  ts,
		"TimeFlowEndMs":    ts,
		"SrcAddr":          "10.0.0.1",
		"DstAddr":          "10.0.0.2",
		"SrcK8S_Namespace": ns,
		"Bytes":            int64(100),
	}
}

func TestRing_EvictionOldestFirst(t *testing.T) {
	r := NewRing(3)
	r.Insert(flow(1000, "a"))
	r.Insert(flow(2000, "b"))
	r.Insert(flow(3000, "c"))
	require.Equal(t, 3, r.Size())
	oldest, newest := r.Window()
	assert.Equal(t, int64(1000), oldest)
	assert.Equal(t, int64(3000), newest)

	r.Insert(flow(4000, "d")) // evicts 1000
	require.Equal(t, 3, r.Size())
	oldest, newest = r.Window()
	assert.Equal(t, int64(2000), oldest)
	assert.Equal(t, int64(4000), newest)

	res := r.Query(QueryFilter{Limit: 10})
	require.Len(t, res.Flows, 3)
	assert.Equal(t, int64(4000), res.Flows[0]["TimeFlowEndMs"])
	assert.Equal(t, int64(3000), res.Flows[1]["TimeFlowEndMs"])
	assert.Equal(t, int64(2000), res.Flows[2]["TimeFlowEndMs"])
}

func TestRing_QueryTimeRangeAndFilter(t *testing.T) {
	r := NewRing(10)
	r.Insert(flow(1000, "ns-a"))
	r.Insert(flow(2000, "ns-b"))
	r.Insert(flow(3000, "ns-a"))

	res := r.Query(QueryFilter{
		StartMs:     1500,
		EndMs:       3500,
		Limit:       10,
		FieldEquals: map[string][]string{"SrcK8S_Namespace": {"ns-a"}},
	})
	require.Len(t, res.Flows, 1)
	assert.Equal(t, int64(3000), res.Flows[0]["TimeFlowEndMs"])
	assert.False(t, res.Truncated)
}

func TestRing_QueryTruncated(t *testing.T) {
	r := NewRing(10)
	for i := 0; i < 5; i++ {
		r.Insert(flow(int64(1000+i), "ns"))
	}
	res := r.Query(QueryFilter{Limit: 2})
	require.Len(t, res.Flows, 2)
	assert.True(t, res.Truncated)
	assert.Equal(t, 5, res.Size)
	assert.Equal(t, 10, res.Capacity)
}

func TestRing_ConcurrentIngestAndQuery(t *testing.T) {
	r := NewRing(1000)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			r.Insert(flow(int64(i+1), "ns"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = r.Query(QueryFilter{Limit: 50})
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()
	assert.Equal(t, 500, r.Size())
}

func TestServer_LocalHTTP(t *testing.T) {
	r := NewRing(10)
	r.Insert(flow(1000, "ns-a"))
	r.Insert(flow(2000, "ns-b"))
	s := NewServer(r, nil, time.Second, ":0")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/flowbuffer/local/flows?limit=1&filter.SrcK8S_Namespace=ns-b")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result QueryResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Len(t, result.Flows, 1)
	assert.Equal(t, "ns-b", result.Flows[0]["SrcK8S_Namespace"])
}

func TestServer_PeerFanInPartialFailure(t *testing.T) {
	local := NewRing(10)
	local.Insert(flow(3000, "local"))

	peerRing := NewRing(10)
	peerRing.Insert(flow(2000, "peer"))
	peerRing.Insert(flow(1000, "peer"))
	peerHTTP := httptest.NewServer(NewServer(peerRing, nil, time.Second, ":0").Handler())
	defer peerHTTP.Close()

	downPeer := "http://127.0.0.1:1" // nothing listening

	s := NewServer(local, StaticPeers{URLs: []string{peerHTTP.URL, downPeer}}, 500*time.Millisecond, ":0")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/flowbuffer/flows?limit=10")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result ClusterQueryResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Len(t, result.Flows, 3)
	assert.EqualValues(t, 3000, result.Flows[0]["TimeFlowEndMs"])
	assert.Equal(t, 1, result.PeersFailed)
	require.NotEmpty(t, result.Warnings)
	assert.Equal(t, WarningPeerQueryFailed, result.Warnings[0].Code)
	assert.Equal(t, int64(1000), result.OldestTimestamp)
	assert.Equal(t, int64(3000), result.NewestTimestamp)
}

func TestServer_ClusterAppliesLimit(t *testing.T) {
	local := NewRing(10)
	local.Insert(flow(5000, "a"))
	peerRing := NewRing(10)
	peerRing.Insert(flow(4000, "b"))
	peerRing.Insert(flow(3000, "c"))
	peerHTTP := httptest.NewServer(NewServer(peerRing, nil, time.Second, ":0").Handler())
	defer peerHTTP.Close()

	s := NewServer(local, StaticPeers{URLs: []string{peerHTTP.URL}}, time.Second, ":0")
	result := s.queryCluster(context.Background(), QueryFilter{Limit: 2})
	require.Len(t, result.Flows, 2)
	assert.True(t, result.Truncated)
	assert.EqualValues(t, 5000, result.Flows[0]["TimeFlowEndMs"])
	assert.EqualValues(t, 4000, result.Flows[1]["TimeFlowEndMs"])
}

func TestNewWriter_Smoke(t *testing.T) {
	utils.InitExitChannel()
	defer utils.CloseExitChannel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	w, err := NewWriter(config.StageParam{
		Name: "fb",
		Write: &config.Write{
			Type: api.FlowBufferType,
			FlowBuffer: &api.WriteFlowBuffer{
				MaxEntries:         100,
				QueryListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
				QueryTimeout:       api.Duration{Duration: time.Second},
			},
		},
	})
	require.NoError(t, err)
	w.Write(flow(1000, "ns"))
	require.Equal(t, 1, w.Ring().Size())

	// Wait briefly for listener
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	for time.Now().Before(deadline) {
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/flowbuffer/local/flows", port))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
