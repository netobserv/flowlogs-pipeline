/*
 * Copyright (C) 2024 IBM, Inc.
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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode/metrics"
	putils "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type mInfoStruct struct {
	genericMetric interface{} // can be a counter, gauge, or histogram pointer
	info          *metrics.Preprocessed
}

type MetricsCommonStruct struct {
	gauges           map[string]mInfoStruct
	counters         map[string]mInfoStruct
	histos           map[string]mInfoStruct
	aggHistos        map[string]mInfoStruct
	mCache           *putils.TimedCache // nil when using Vec-native TTL (Prometheus path)
	mCacheLenMetric  prometheus.Gauge
	metricsProcessed prometheus.Counter
	metricsDropped   prometheus.Counter
	errorsCounter    *prometheus.CounterVec
	expiryTime       time.Duration
	maxEntries       int
	exitChan         <-chan struct{}
}

type MetricsCommonInterface interface {
	GetCacheEntry(entryLabels map[string]string, m interface{}) interface{}
	ProcessCounter(m interface{}, name string, labels map[string]string, value float64) error
	ProcessGauge(m interface{}, name string, labels map[string]string, value float64, lvs []string) error
	ProcessHist(m interface{}, name string, labels map[string]string, value float64) error
	ProcessAggHist(m interface{}, name string, labels map[string]string, value []float64) error
}

var (
	metricsProcessed = operational.DefineMetric(
		"metrics_processed",
		"Number of metrics processed",
		operational.TypeCounter,
		"stage",
	)
	metricsDropped = operational.DefineMetric(
		"metrics_dropped",
		"Number of metrics dropped",
		operational.TypeCounter,
		"stage",
	)
	encodePromErrors = operational.DefineMetric(
		"encode_prom_errors",
		"Total errors during metrics generation",
		operational.TypeCounter,
		"error", "metric", "key",
	)
	mChacheLen = operational.DefineMetric(
		"encode_prom_metrics_reported",
		"Total number of prometheus metrics reported by this stage",
		operational.TypeGauge,
		"stage",
	)
)

func (m *MetricsCommonStruct) AddCounter(name string, g interface{}, info *metrics.Preprocessed) {
	mStruct := mInfoStruct{genericMetric: g, info: info}
	m.counters[name] = mStruct
}

func (m *MetricsCommonStruct) AddGauge(name string, g interface{}, info *metrics.Preprocessed) {
	mStruct := mInfoStruct{genericMetric: g, info: info}
	m.gauges[name] = mStruct
}

func (m *MetricsCommonStruct) AddHist(name string, g interface{}, info *metrics.Preprocessed) {
	mStruct := mInfoStruct{genericMetric: g, info: info}
	m.histos[name] = mStruct
}

func (m *MetricsCommonStruct) AddAggHist(name string, g interface{}, info *metrics.Preprocessed) {
	mStruct := mInfoStruct{genericMetric: g, info: info}
	m.aggHistos[name] = mStruct
}

func (m *MetricsCommonStruct) MetricCommonEncode(mci MetricsCommonInterface, metricRecord config.GenericMap) {
	log.Tracef("entering MetricCommonEncode. metricRecord = %v", metricRecord)

	// Process counters
	for _, mInfo := range m.counters {
		labelSets, value := m.prepareMetric(mci, metricRecord, mInfo.info, mInfo.genericMetric)
		if labelSets == nil {
			continue
		}
		for _, labels := range labelSets {
			err := mci.ProcessCounter(mInfo.genericMetric, mInfo.info.Name, labels.lMap, value)
			if err != nil {
				log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
				m.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
				continue
			}
			m.metricsProcessed.Inc()
		}
	}

	// Process gauges
	for _, mInfo := range m.gauges {
		labelSets, value := m.prepareMetric(mci, metricRecord, mInfo.info, mInfo.genericMetric)
		if labelSets == nil {
			continue
		}
		for _, labels := range labelSets {
			err := mci.ProcessGauge(mInfo.genericMetric, mInfo.info.Name, labels.lMap, value, labels.values)
			if err != nil {
				log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
				m.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
				continue
			}
			m.metricsProcessed.Inc()
		}
	}

	// Process histograms
	for _, mInfo := range m.histos {
		labelSets, value := m.prepareMetric(mci, metricRecord, mInfo.info, mInfo.genericMetric)
		if labelSets == nil {
			continue
		}
		for _, labels := range labelSets {
			err := mci.ProcessHist(mInfo.genericMetric, mInfo.info.Name, labels.lMap, value)
			if err != nil {
				log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
				m.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
				continue
			}
			m.metricsProcessed.Inc()
		}
	}

	// Process pre-aggregated histograms
	for _, mInfo := range m.aggHistos {
		labelSets, values := m.prepareAggHisto(mci, metricRecord, mInfo.info, mInfo.genericMetric)
		if labelSets == nil {
			continue
		}
		for _, labels := range labelSets {
			err := mci.ProcessAggHist(mInfo.genericMetric, mInfo.info.Name, labels.lMap, values)
			if err != nil {
				log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
				m.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
				continue
			}
			m.metricsProcessed.Inc()
		}
	}
}

func (m *MetricsCommonStruct) prepareMetric(mci MetricsCommonInterface, flow config.GenericMap, info *metrics.Preprocessed, mv interface{}) ([]labelsKeyAndMap, float64) {
	flatParts := info.GenerateFlatParts(flow)
	ok, flatParts := info.ApplyFilters(flow, flatParts)
	if !ok {
		return nil, 0
	}

	val := m.extractGenericValue(flow, info)
	if val == nil {
		return nil, 0
	}
	floatVal, err := utils.ConvertToFloat64(val)
	if err != nil {
		m.errorsCounter.WithLabelValues("ValueConversionError", info.Name, info.ValueKey).Inc()
		return nil, 0
	}
	if info.ValueScale != 0 {
		floatVal /= info.ValueScale
	}

	labelSets := extractLabels(flow, flatParts, info)
	if m.mCache != nil {
		for _, ls := range labelSets {
			ok := m.mCache.UpdateCacheEntry(ls.values, func() interface{} {
				return mci.GetCacheEntry(ls.lMap, mv)
			})
			if !ok {
				m.metricsDropped.Inc()
				return nil, 0
			}
		}
	} else if m.maxEntries > 0 {
		// Without cache, enforce max entries via Vec child count
		total := m.countVecChildren()
		if total >= m.maxEntries {
			m.metricsDropped.Inc()
			return nil, 0
		}
	}
	return labelSets, floatVal
}

func (m *MetricsCommonStruct) prepareAggHisto(mci MetricsCommonInterface, flow config.GenericMap, info *metrics.Preprocessed, mc interface{}) ([]labelsKeyAndMap, []float64) {
	flatParts := info.GenerateFlatParts(flow)
	ok, flatParts := info.ApplyFilters(flow, flatParts)
	if !ok {
		return nil, nil
	}

	val := m.extractGenericValue(flow, info)
	if val == nil {
		return nil, nil
	}
	values, ok := val.([]float64)
	if !ok {
		m.errorsCounter.WithLabelValues("HistoValueConversionError", info.Name, info.ValueKey).Inc()
		return nil, nil
	}

	labelSets := extractLabels(flow, flatParts, info)
	if m.mCache != nil {
		for _, ls := range labelSets {
			ok := m.mCache.UpdateCacheEntry(ls.values, func() interface{} {
				return mci.GetCacheEntry(ls.lMap, mc)
			})
			if !ok {
				m.metricsDropped.Inc()
				return nil, nil
			}
		}
	} else if m.maxEntries > 0 {
		total := m.countVecChildren()
		if total >= m.maxEntries {
			m.metricsDropped.Inc()
			return nil, nil
		}
	}
	return labelSets, values
}

func (m *MetricsCommonStruct) extractGenericValue(flow config.GenericMap, info *metrics.Preprocessed) interface{} {
	if info.ValueKey == "" {
		// No value key means it's a records / flows counter (1 flow = 1 increment), so just return 1
		return 1
	}
	val, found := flow[info.ValueKey]
	if !found {
		// No value might mean 0 for counters, to keep storage lightweight - it can safely be ignored
		return nil
	}
	return val
}

type labelsKeyAndMap struct {
	values []string
	lMap   map[string]string
}

// extractLabels takes the flow and a single metric definition as input.
// It returns the flat labels maps (label names and values).
// Most of the time it will return a single map; it may return several of them when the parsed flow fields are lists (e.g. "interfaces").
func extractLabels(flow config.GenericMap, flatParts []config.GenericMap, info *metrics.Preprocessed) []labelsKeyAndMap {
	common := newLabelKeyAndMap(info.Name, flow, info.MappedLabels)
	if len(flatParts) == 0 {
		return []labelsKeyAndMap{common}
	}
	all := make([]labelsKeyAndMap, 0, len(flatParts))
	for _, fp := range flatParts {
		ls := newLabelKeyAndMap(info.Name, fp, info.FlattenedLabels)
		ls.values = append(ls.values, common.values...)
		for k, v := range common.lMap {
			ls.lMap[k] = v
		}
		all = append(all, ls)
	}
	return all
}

func newLabelKeyAndMap(name string, part config.GenericMap, labels []metrics.MappedLabel) labelsKeyAndMap {
	values := make([]string, 0, len(labels)+1)
	values = append(values, name)
	m := make(map[string]string, len(labels))
	for _, t := range labels {
		value := ""
		if v, ok := part[t.Source]; ok {
			value = utils.ConvertToString(v)
		}
		values = append(values, value)
		m[t.Target] = value
	}
	return labelsKeyAndMap{values: values, lMap: m}
}

func (m *MetricsCommonStruct) cleanupExpiredEntriesLoop(callback putils.CacheCallback) {
	ticker := time.NewTicker(m.expiryTime)
	for {
		select {
		case <-m.exitChan:
			log.Debugf("exiting cleanupExpiredEntriesLoop because of signal")
			return
		case <-ticker.C:
			if m.mCache != nil {
				m.mCache.CleanupExpiredEntries(m.expiryTime, callback)
			} else {
				m.cleanupVecExpired()
			}
		}
	}
}

// cleanupVecExpired calls CleanupExpired on each Prometheus Vec and updates the gauge.
func (m *MetricsCommonStruct) cleanupVecExpired() {
	allMaps := []map[string]mInfoStruct{m.counters, m.gauges, m.histos, m.aggHistos}
	for _, store := range allMaps {
		for _, mInfo := range store {
			if vec, ok := mInfo.genericMetric.(interface{ CleanupExpired() int }); ok {
				vec.CleanupExpired()
			}
		}
	}
	m.mCacheLenMetric.Set(float64(m.countVecChildren()))
}

// countVecChildren returns the total number of children across all Vecs.
func (m *MetricsCommonStruct) countVecChildren() int {
	total := 0
	allMaps := []map[string]mInfoStruct{m.counters, m.gauges, m.histos, m.aggHistos}
	for _, store := range allMaps {
		for _, mInfo := range store {
			if c, ok := mInfo.genericMetric.(prometheus.Collector); ok {
				ch := make(chan prometheus.Metric, 1000)
				go func() {
					c.Collect(ch)
					close(ch)
				}()
				for range ch {
					total++
				}
			}
		}
	}
	return total
}

func (m *MetricsCommonStruct) cleanupInfoStructs() {
	m.gauges = map[string]mInfoStruct{}
	m.counters = map[string]mInfoStruct{}
	m.histos = map[string]mInfoStruct{}
	m.aggHistos = map[string]mInfoStruct{}
}

func NewMetricsCommonStruct(opMetrics *operational.Metrics, maxCacheEntries int, name string, expiryTime api.Duration, callback putils.CacheCallback) *MetricsCommonStruct {
	mChacheLenMetric := opMetrics.NewGauge(&mChacheLen, name)
	m := &MetricsCommonStruct{
		mCache:           putils.NewTimedCache(maxCacheEntries, mChacheLenMetric),
		mCacheLenMetric:  mChacheLenMetric,
		metricsProcessed: opMetrics.NewCounter(&metricsProcessed, name),
		metricsDropped:   opMetrics.NewCounter(&metricsDropped, name),
		errorsCounter:    opMetrics.NewCounterVec(&encodePromErrors),
		expiryTime:       expiryTime.Duration,
		maxEntries:       maxCacheEntries,
		exitChan:         putils.ExitChannel(),
		gauges:           map[string]mInfoStruct{},
		counters:         map[string]mInfoStruct{},
		histos:           map[string]mInfoStruct{},
		aggHistos:        map[string]mInfoStruct{},
	}
	go m.cleanupExpiredEntriesLoop(callback)
	return m
}

// NewMetricsCommonStructWithVecTTL creates a MetricsCommonStruct that relies on
// Prometheus Vec TTL (via prometheus.TTLRegistry in encode_prom) instead of an
// external TimedCache.
func NewMetricsCommonStructWithVecTTL(opMetrics *operational.Metrics, maxCacheEntries int, name string, expiryTime api.Duration) *MetricsCommonStruct {
	mChacheLenMetric := opMetrics.NewGauge(&mChacheLen, name)
	m := &MetricsCommonStruct{
		mCache:           nil, // no external cache needed
		mCacheLenMetric:  mChacheLenMetric,
		metricsProcessed: opMetrics.NewCounter(&metricsProcessed, name),
		metricsDropped:   opMetrics.NewCounter(&metricsDropped, name),
		errorsCounter:    opMetrics.NewCounterVec(&encodePromErrors),
		expiryTime:       expiryTime.Duration,
		maxEntries:       maxCacheEntries,
		exitChan:         putils.ExitChannel(),
		gauges:           map[string]mInfoStruct{},
		counters:         map[string]mInfoStruct{},
		histos:           map[string]mInfoStruct{},
		aggHistos:        map[string]mInfoStruct{},
	}
	return m
}

// StartCleanupLoop launches the background goroutine that periodically cleans
// up expired Vec entries. Must be called after all initial metrics are registered
// to avoid races between the cleanup goroutine and metric setup.
func (m *MetricsCommonStruct) StartCleanupLoop() {
	go m.cleanupExpiredEntriesLoop(nil)
}
