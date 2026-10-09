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
	"sync"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode/metrics"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func gatheredTTLMetric(t *testing.T, enc *Prometheus, name, label string) *dto.Metric {
	t.Helper()
	families, err := enc.Gatherer().Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() == name {
			for _, metric := range family.Metric {
				for _, pair := range metric.Label {
					if pair.GetValue() == label {
						return metric
					}
				}
			}
		}
	}
	t.Fatalf("metric %s with label %q not found", name, label)
	return nil
}

func ttlCounterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	var metric dto.Metric
	require.NoError(t, counter.Write(&metric))
	return metric.GetCounter().GetValue()
}

func Test_TTL_MaxMetricsRefreshAtCapacity(t *testing.T) {
	for _, typ := range []api.MetricEncodeOperationEnum{api.MetricCounter, api.MetricGauge, api.MetricHistogram, api.MetricAggHistogram} {
		t.Run(string(typ), func(t *testing.T) {
			ttl := 300 * time.Millisecond
			params := api.PromEncode{
				Prefix: "flp_", ExpiryTime: ttlDuration(ttl), MaxMetrics: 2,
				Metrics: []api.MetricsItem{{Name: "sample", Type: typ, ValueKey: "value", Labels: []string{"series"}}},
			}
			enc, err := initProm(&params)
			require.NoError(t, err)
			flow := func(label string, value float64) config.GenericMap {
				var v interface{} = value
				if typ == api.MetricAggHistogram {
					v = []float64{value, value * 2}
				}
				return config.GenericMap{"series": label, "value": v}
			}
			enc.Encode(flow("active", 2))
			enc.Encode(flow("idle", 2))
			enc.Encode(flow("rejected", 2))
			require.Equal(t, float64(1), ttlCounterValue(t, enc.metricCommon.metricsDropped))
			for range 4 {
				enc.Encode(flow("active", 3))
				time.Sleep(ttl / 3)
			}
			metric := gatheredTTLMetric(t, enc, "flp_sample", "active")
			switch typ {
			case api.MetricCounter:
				require.Equal(t, float64(14), metric.GetCounter().GetValue())
			case api.MetricGauge:
				require.Equal(t, float64(3), metric.GetGauge().GetValue())
			case api.MetricHistogram:
				require.Equal(t, uint64(5), metric.GetHistogram().GetSampleCount())
				require.Equal(t, float64(14), metric.GetHistogram().GetSampleSum())
			case api.MetricAggHistogram:
				require.Equal(t, uint64(10), metric.GetHistogram().GetSampleCount())
				require.Equal(t, float64(42), metric.GetHistogram().GetSampleSum())
			}
			require.Equal(t, float64(1), ttlCounterValue(t, enc.metricCommon.metricsDropped), "only a new child should be dropped")
			enc.Encode(flow("replacement", 7))
			require.Equal(t, 2, enc.metricCommon.countVecChildren())
			_ = gatheredTTLMetric(t, enc, "flp_sample", "replacement")
		})
	}
}

func Test_TTL_MaxMetricsFlattenedLabels(t *testing.T) {
	params := api.PromEncode{
		Prefix: "flp_", ExpiryTime: ttlDuration(time.Hour), MaxMetrics: 2,
		Metrics: []api.MetricsItem{{Name: "samples_total", Type: api.MetricCounter, Labels: []string{"interfaces"}, Flatten: []string{"interfaces"}}},
	}
	enc, err := initProm(&params)
	require.NoError(t, err)
	enc.Encode(config.GenericMap{"interfaces": []string{"eth0", "eth1", "eth2"}})
	require.Equal(t, 2, enc.metricCommon.countVecChildren(), "one flattened record must not exceed the cap")
	enc.Encode(config.GenericMap{"interfaces": []string{"eth0", "eth2"}})
	require.Equal(t, float64(2), gatheredTTLMetric(t, enc, "flp_samples_total", "eth0").GetCounter().GetValue())
	require.Equal(t, float64(2), ttlCounterValue(t, enc.metricCommon.metricsDropped))
}

func Test_TTL_MaxMetricsReleaseOnConfigurationChange(t *testing.T) {
	params := api.PromEncode{
		Prefix: "flp_", ExpiryTime: ttlDuration(time.Hour), MaxMetrics: 1,
		Metrics: []api.MetricsItem{{Name: "first_total", Type: api.MetricCounter, Labels: []string{"series"}}},
	}
	enc, err := initProm(&params)
	require.NoError(t, err)
	enc.Encode(config.GenericMap{"series": "first"})
	require.Len(t, enc.metricCommon.admission.entries, 1)
	enc.cleanDeletedMetrics(api.PromEncode{})
	require.Empty(t, enc.metricCommon.admission.entries)
	enc.resetRegistry()
	enc.Encode(config.GenericMap{"series": "replacement"})
	require.Equal(t, float64(1), gatheredTTLMetric(t, enc, "flp_first_total", "replacement").GetCounter().GetValue())
	enc.resetRegistry()
	require.Empty(t, enc.metricCommon.admission.entries)
}

func Test_TTL_ConcurrentConfigurationCleanup(t *testing.T) {
	params := api.PromEncode{
		Prefix: "flp_", ExpiryTime: ttlDuration(time.Hour), MaxMetrics: 1,
		Metrics: []api.MetricsItem{{Name: "samples_total", Type: api.MetricCounter, Labels: []string{"series"}}},
	}
	enc, err := initProm(&params)
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			enc.metricCommon.cleanupVecExpired()
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			enc.Encode(config.GenericMap{"series": "active"})
		}
	}()
	for range 100 {
		enc.cleanDeletedMetrics(api.PromEncode{})
		enc.metricCommon.cleanupInfoStructs()
		enc.addCounter("flp_samples_total", metrics.Preprocess(&params.Metrics[0]))
	}
	wg.Wait()
	enc.Encode(config.GenericMap{"series": "active"})
	require.Equal(t, 1, enc.metricCommon.countVecChildren())
	require.Len(t, enc.metricCommon.admission.entries, 1)
}
