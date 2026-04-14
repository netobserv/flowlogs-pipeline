package encode

import (
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func benchPromParams(ttl time.Duration, nLabels int) api.PromEncode {
	labels := make([]string, nLabels)
	for i := range labels {
		labels[i] = fmt.Sprintf("label_%d", i)
	}
	return api.PromEncode{
		Prefix:     "bench_",
		ExpiryTime: api.Duration{Duration: ttl},
		Metrics: []api.MetricsItem{
			{Name: "bytes_total", Type: "counter", ValueKey: "bytes", Labels: labels},
			{Name: "latency_seconds", Type: "histogram", ValueKey: "latency", Labels: labels},
		},
	}
}

func buildBenchFlow(nLabels, cardinality int) config.GenericMap {
	m := config.GenericMap{
		"bytes":   rand.Intn(10000),
		"latency": rand.Float64(),
	}
	for i := 0; i < nLabels; i++ {
		m[fmt.Sprintf("label_%d", i)] = "val_" + strconv.Itoa(rand.Intn(cardinality))
	}
	return m
}

// initOldPathProm creates a Prometheus encoder using the old TimedCache path.
func initOldPathProm(tb testing.TB, params *api.PromEncode) *Prometheus {
	tb.Helper()
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	http.DefaultServeMux = http.NewServeMux()
	opMetrics := operational.NewMetrics(&config.MetricsSettings{})

	expiryTime := params.ExpiryTime
	if expiryTime.Duration == 0 {
		expiryTime.Duration = defaultExpiryTime
	}

	w := &Prometheus{
		cfg:        params,
		registerer: reg,
		updateChan: make(chan config.StageParam),
	}

	metricCommon := NewMetricsCommonStruct(opMetrics, params.MaxMetrics, "bench", expiryTime, func(cleanupFunc interface{}) {
		if f, ok := cleanupFunc.(func()); ok {
			f()
		}
	})
	w.metricCommon = metricCommon

	for i := range params.Metrics {
		mCfg := &params.Metrics[i]
		fullMetricName := params.Prefix + mCfg.Name
		mInfo := metrics.Preprocess(mCfg)
		switch mCfg.Type {
		case api.MetricCounter:
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: mInfo.Help}, mInfo.TargetLabels())
			w.metricCommon.AddCounter(fullMetricName, counter, mInfo)
			reg.MustRegister(counter)
		case api.MetricHistogram:
			histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: mInfo.Help}, mInfo.TargetLabels())
			w.metricCommon.AddHist(fullMetricName, histogram, mInfo)
			reg.MustRegister(histogram)
		}
	}
	return w
}

// initNewPathProm creates a Prometheus encoder using the new Vec-TTL path.
func initNewPathProm(tb testing.TB, params *api.PromEncode) *Prometheus {
	tb.Helper()
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	http.DefaultServeMux = http.NewServeMux()
	opMetrics := operational.NewMetrics(&config.MetricsSettings{})
	enc, err := NewEncodeProm(opMetrics, config.StageParam{Encode: &config.Encode{Prom: params}})
	require.NoError(tb, err)
	return enc.(*Prometheus)
}

// BenchmarkEncodeVecTTL benchmarks the new Vec-TTL path (no TimedCache).
func BenchmarkEncodeVecTTL(b *testing.B) {
	params := benchPromParams(60*time.Second, 5)
	prom := initNewPathProm(b, &params)
	flows := make([]config.GenericMap, 1000)
	for i := range flows {
		flows[i] = buildBenchFlow(5, 20)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prom.Encode(flows[i%len(flows)])
	}
}

// BenchmarkEncodeOldCache benchmarks the old TimedCache path for comparison.
func BenchmarkEncodeOldCache(b *testing.B) {
	params := benchPromParams(60*time.Second, 5)
	prom := initOldPathProm(b, &params)
	flows := make([]config.GenericMap, 1000)
	for i := range flows {
		flows[i] = buildBenchFlow(5, 20)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prom.Encode(flows[i%len(flows)])
	}
}

// TestCleanupPerformance measures cleanup cost for both paths at different
// cardinalities. Uses a test instead of a benchmark to avoid the sleep-per-iteration
// problem (each run needs entries to expire before cleanup can proceed).
func TestCleanupPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cleanup perf test in short mode")
	}

	for _, n := range []int{100, 1000, 10000} {
		t.Run(fmt.Sprintf("series_%d", n), func(t *testing.T) {
			ttl := 100 * time.Millisecond
			nLabels := 5
			iterations := 20

			flows := make([]config.GenericMap, n)
			for i := 0; i < n; i++ {
				m := config.GenericMap{"bytes": 100, "latency": 0.5}
				for l := 0; l < nLabels; l++ {
					m[fmt.Sprintf("label_%d", l)] = fmt.Sprintf("v%d", i)
				}
				flows[i] = m
			}

			// --- Old path ---
			var oldTotal time.Duration
			for iter := 0; iter < iterations; iter++ {
				params := benchPromParams(ttl, nLabels)
				prom := initOldPathProm(t, &params)
				for _, f := range flows {
					prom.Encode(f)
				}
				time.Sleep(ttl + 20*time.Millisecond)

				cleanupCb := func(entry interface{}) {
					if f, ok := entry.(func()); ok {
						f()
					}
				}
				start := time.Now()
				prom.metricCommon.mCache.CleanupExpiredEntries(ttl, cleanupCb)
				oldTotal += time.Since(start)
			}

			// --- New path ---
			var newTotal time.Duration
			for iter := 0; iter < iterations; iter++ {
				params := benchPromParams(ttl, nLabels)
				prom := initNewPathProm(t, &params)
				for _, f := range flows {
					prom.Encode(f)
				}
				time.Sleep(ttl + 20*time.Millisecond)

				start := time.Now()
				prom.metricCommon.cleanupVecExpired()
				newTotal += time.Since(start)
			}

			oldAvg := oldTotal / time.Duration(iterations)
			newAvg := newTotal / time.Duration(iterations)
			t.Logf("Cleanup %d series (avg of %d runs):", n, iterations)
			t.Logf("  Old (TimedCache + Delete callbacks): %v", oldAvg)
			t.Logf("  New (Vec CleanupExpired):            %v", newAvg)
			if oldAvg > newAvg {
				t.Logf("  Speedup: %.1fx faster", float64(oldAvg)/float64(newAvg))
			} else {
				t.Logf("  Slowdown: %.1fx slower", float64(newAvg)/float64(oldAvg))
			}
		})
	}
}

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// TestMemoryFootprint measures and reports steady-state memory for both
// approaches at different cardinalities.
func TestMemoryFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory footprint test in short mode")
	}

	for _, cardinality := range []int{1000, 5000, 10000} {
		t.Run(fmt.Sprintf("cardinality_%d", cardinality), func(t *testing.T) {
			nLabels := 5

			// Pre-generate deterministic flows for exact cardinality
			flows := make([]config.GenericMap, cardinality)
			for i := 0; i < cardinality; i++ {
				m := config.GenericMap{
					"bytes":   100,
					"latency": 0.5,
				}
				for l := 0; l < nLabels; l++ {
					m[fmt.Sprintf("label_%d", l)] = fmt.Sprintf("v%d", i)
				}
				flows[i] = m
			}

			// --- Measure old path ---
			params1 := benchPromParams(60*time.Second, nLabels)
			promOld := initOldPathProm(t, &params1)
			runtime.GC()
			runtime.GC()
			before := heapInUse()
			for _, f := range flows {
				promOld.Encode(f)
			}
			afterOld := heapInUse()
			memOld := int64(afterOld) - int64(before)
			if memOld < 0 {
				memOld = 0
			}

			// Clear references
			promOld = nil
			runtime.GC()
			runtime.GC()

			// --- Measure new path ---
			params2 := benchPromParams(60*time.Second, nLabels)
			promNew := initNewPathProm(t, &params2)
			runtime.GC()
			runtime.GC()
			before = heapInUse()
			for _, f := range flows {
				promNew.Encode(f)
			}
			afterNew := heapInUse()
			memNew := int64(afterNew) - int64(before)
			if memNew < 0 {
				memNew = 0
			}

			t.Logf("Cardinality %d (2 metrics x %d labels):", cardinality, nLabels)
			t.Logf("  Old (TimedCache):  %d KB  (%d bytes/series)", memOld/1024, memOld/int64(cardinality))
			t.Logf("  New (Vec TTL):     %d KB  (%d bytes/series)", memNew/1024, memNew/int64(cardinality))
			if memOld > memNew {
				saved := memOld - memNew
				pct := float64(saved) / float64(memOld) * 100
				t.Logf("  Saved:             %d KB  (%.1f%%)", saved/1024, pct)
			}

			_ = promNew
		})
	}
}
