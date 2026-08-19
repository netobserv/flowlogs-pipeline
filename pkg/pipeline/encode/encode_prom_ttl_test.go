/*
 * Copyright (C) 2026 Red Hat, Inc.
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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

// netObserv-style labels used in production FLP prom encode configs.
const (
	lblSrcNS = "SrcK8S_Namespace"
	lblDstNS = "DstK8S_Namespace"
)

func ttlDuration(d time.Duration) api.Duration {
	return api.Duration{Duration: d}
}

func netObservFlow(srcNS, dstNS string, bytes, packets int, latency float64) config.GenericMap {
	return config.GenericMap{
		lblSrcNS:        srcNS,
		lblDstNS:        dstNS,
		"Bytes":         bytes,
		"Packets":       packets,
		"TimeFlowRttNs": latency,
	}
}

func Test_TTL_RefreshKeepsActiveSeries(t *testing.T) {
	// Active flows (re-encoded before expiry) must stay exposed; idle ones expire.
	ttl := 300 * time.Millisecond
	params := api.PromEncode{
		Prefix:     "flp_",
		ExpiryTime: ttlDuration(ttl),
		Metrics: []api.MetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "Bytes",
			Labels:   []string{lblSrcNS, lblDstNS},
		}},
	}

	enc, err := initProm(&params)
	require.NoError(t, err)

	active := netObservFlow("app", "db", 100, 1, 0)
	idle := netObservFlow("app", "cache", 50, 1, 0)
	enc.Encode(active)
	enc.Encode(idle)

	exposed := test.ReadExposedMetrics(t, enc.server)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="db",SrcK8S_Namespace="app"}`)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="cache",SrcK8S_Namespace="app"}`)

	time.Sleep(ttl / 2)
	// Refresh only the active series (and bump the counter).
	enc.Encode(netObservFlow("app", "db", 25, 1, 0))

	time.Sleep(ttl*2/3 + 50*time.Millisecond)

	exposed = test.ReadExposedMetrics(t, enc.server)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="db",SrcK8S_Namespace="app"} 125`)
	require.NotContains(t, exposed, `DstK8S_Namespace="cache"`)
}

func Test_TTL_PartialExpiryAcrossNamespaces(t *testing.T) {
	ttl := 300 * time.Millisecond
	params := api.PromEncode{
		Prefix:     "flp_",
		ExpiryTime: ttlDuration(ttl),
		Metrics: []api.MetricsItem{{
			Name:     "packets_total",
			Type:     "counter",
			ValueKey: "Packets",
			Labels:   []string{lblSrcNS, lblDstNS},
		}},
	}

	enc, err := initProm(&params)
	require.NoError(t, err)

	flows := []config.GenericMap{
		netObservFlow("ns-a", "ns-b", 0, 3, 0),
		netObservFlow("ns-c", "ns-d", 0, 7, 0),
		netObservFlow("ns-e", "ns-f", 0, 11, 0),
	}
	for _, f := range flows {
		enc.Encode(f)
	}
	require.Equal(t, 3, enc.metricCommon.countVecChildren())

	time.Sleep(ttl / 2)
	enc.Encode(netObservFlow("ns-a", "ns-b", 0, 2, 0))
	enc.Encode(netObservFlow("ns-e", "ns-f", 0, 1, 0))

	time.Sleep(ttl*2/3 + 50*time.Millisecond)
	exposed := test.ReadExposedMetrics(t, enc.server)

	require.Contains(t, exposed, `flp_packets_total{DstK8S_Namespace="ns-b",SrcK8S_Namespace="ns-a"} 5`)
	require.Contains(t, exposed, `flp_packets_total{DstK8S_Namespace="ns-f",SrcK8S_Namespace="ns-e"} 12`)
	require.NotContains(t, exposed, `SrcK8S_Namespace="ns-c"`)
	require.Equal(t, 2, enc.metricCommon.countVecChildren())
}

func Test_TTL_CounterGaugeHistogramExpireOnScrape(t *testing.T) {
	// NetObserv exposes counters, gauges, and histograms together; all must honour expiryTime.
	ttl := 250 * time.Millisecond
	params := api.PromEncode{
		Prefix:     "flp_",
		ExpiryTime: ttlDuration(ttl),
		Metrics: []api.MetricsItem{
			{
				Name:     "bytes_total",
				Type:     "counter",
				ValueKey: "Bytes",
				Labels:   []string{lblSrcNS, lblDstNS},
			},
			{
				Name:     "bytes",
				Type:     "gauge",
				ValueKey: "Bytes",
				Labels:   []string{lblSrcNS, lblDstNS},
			},
			{
				Name:     "rtt_seconds",
				Type:     "histogram",
				ValueKey: "TimeFlowRttNs",
				Labels:   []string{lblSrcNS, lblDstNS},
				Buckets:  []float64{0.001, 0.01, 0.1, 1},
			},
		},
	}

	enc, err := initProm(&params)
	require.NoError(t, err)

	enc.Encode(netObservFlow("frontend", "backend", 42, 1, 0.005))
	exposed := test.ReadExposedMetrics(t, enc.server)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="backend",SrcK8S_Namespace="frontend"} 42`)
	require.Contains(t, exposed, `flp_bytes{DstK8S_Namespace="backend",SrcK8S_Namespace="frontend"} 42`)
	require.Contains(t, exposed, `flp_rtt_seconds_count{DstK8S_Namespace="backend",SrcK8S_Namespace="frontend"} 1`)

	time.Sleep(ttl + 100*time.Millisecond)
	exposed = test.ReadExposedMetrics(t, enc.server)

	require.NotContains(t, exposed, `flp_bytes_total{`)
	require.NotContains(t, exposed, `flp_bytes{`)
	require.NotContains(t, exposed, `flp_rtt_seconds_count{`)
	require.Equal(t, 0, enc.metricCommon.countVecChildren())
}

func Test_TTL_MaxMetricsFreedAfterExpiry(t *testing.T) {
	// When maxMetrics is hit, new series are dropped; after TTL cleanup, slots reopen.
	ttl := 250 * time.Millisecond
	params := api.PromEncode{
		Prefix:     "flp_",
		ExpiryTime: ttlDuration(ttl),
		MaxMetrics: 2,
		Metrics: []api.MetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "Bytes",
			Labels:   []string{lblSrcNS, lblDstNS},
		}},
	}

	enc, err := initProm(&params)
	require.NoError(t, err)

	enc.Encode(netObservFlow("ns1", "ns2", 1, 1, 0))
	enc.Encode(netObservFlow("ns3", "ns4", 1, 1, 0))
	require.Equal(t, 2, enc.metricCommon.countVecChildren())

	enc.Encode(netObservFlow("ns5", "ns6", 1, 1, 0))
	require.Equal(t, 2, enc.metricCommon.countVecChildren(), "third series must be dropped at maxMetrics")
	exposed := test.ReadExposedMetrics(t, enc.server)
	require.NotContains(t, exposed, `SrcK8S_Namespace="ns5"`)

	time.Sleep(ttl + 100*time.Millisecond)
	_ = test.ReadExposedMetrics(t, enc.server) // Gather cleans expired children
	require.Equal(t, 0, enc.metricCommon.countVecChildren())

	enc.Encode(netObservFlow("ns5", "ns6", 9, 1, 0))
	enc.Encode(netObservFlow("ns7", "ns8", 8, 1, 0))
	require.Equal(t, 2, enc.metricCommon.countVecChildren())

	exposed = test.ReadExposedMetrics(t, enc.server)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="ns6",SrcK8S_Namespace="ns5"} 9`)
	require.Contains(t, exposed, `flp_bytes_total{DstK8S_Namespace="ns8",SrcK8S_Namespace="ns7"} 8`)
}

func Test_TTL_BackgroundCleanupLoop(t *testing.T) {
	// StartCleanupLoop (used by NewEncodeProm) must expire series without an explicit scrape.
	ttl := 200 * time.Millisecond
	params := api.PromEncode{
		Prefix:     "flp_",
		ExpiryTime: ttlDuration(ttl),
		Metrics: []api.MetricsItem{{
			Name:     "bytes_total",
			Type:     "counter",
			ValueKey: "Bytes",
			Labels:   []string{lblSrcNS},
		}},
	}

	enc, err := initProm(&params)
	require.NoError(t, err)

	enc.Encode(config.GenericMap{lblSrcNS: "only-ns", "Bytes": 3})
	require.Equal(t, 1, enc.metricCommon.countVecChildren())

	require.Eventually(t, func() bool {
		return enc.metricCommon.countVecChildren() == 0
	}, 3*ttl, 50*time.Millisecond, "background cleanup loop should drop expired Vec children")
}
