/*
 * Copyright (C) 2021 IBM, Inc.
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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const defaultExpiryTime = 120

type gaugeInfo struct {
	gauge *prometheus.GaugeVec
	info  api.PromMetricsItem
}

type counterInfo struct {
	counter *prometheus.CounterVec
	info    api.PromMetricsItem
}

type histoInfo struct {
	histo *prometheus.HistogramVec
	info  api.PromMetricsItem
}

type EncodeProm struct {
	gauges           []gaugeInfo
	counters         []counterInfo
	histos           []histoInfo
	aggHistos        []histoInfo
	expiryTime       int64
	mCache           *utils.TimedCache
	exitChan         <-chan struct{}
	server           *http.Server
	tlsConfig        *api.PromTLSConf
	metricsProcessed prometheus.Counter
	errorsCounter    *prometheus.CounterVec
}

var (
	metricsProcessed = operational.DefineMetric(
		"metrics_processed",
		"Number of metrics processed",
		operational.TypeCounter,
		"stage",
	)
	encodePromErrors = operational.DefineMetric(
		"encode_prom_errors",
		"Total errors during metrics generation",
		operational.TypeCounter,
		"error", "metric", "key",
	)
)

// Encode encodes a metric before being stored
func (e *EncodeProm) Encode(metricRecord config.GenericMap) {
	log.Debugf("entering EncodeMetric. metricRecord = %v", metricRecord)

	// Process counters
	for _, mInfo := range e.counters {
		labels, value := e.prepareMetric(metricRecord, &mInfo.info, mInfo.counter.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.counter.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			e.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Add(value)
		e.metricsProcessed.Inc()
	}

	// Process gauges
	for _, mInfo := range e.gauges {
		labels, value := e.prepareMetric(metricRecord, &mInfo.info, mInfo.gauge.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.gauge.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			e.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Set(value)
		e.metricsProcessed.Inc()
	}

	// Process histograms
	for _, mInfo := range e.histos {
		labels, value := e.prepareMetric(metricRecord, &mInfo.info, mInfo.histo.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.histo.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			e.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Observe(value)
		e.metricsProcessed.Inc()
	}

	// Process pre-aggregated histograms
	for _, mInfo := range e.aggHistos {
		labels, values := e.prepareAggHisto(metricRecord, &mInfo.info, mInfo.histo.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.histo.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			e.errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		for _, v := range values {
			m.Observe(v)
		}
		e.metricsProcessed.Inc()
	}
}

func (e *EncodeProm) prepareMetric(flow config.GenericMap, info *api.PromMetricsItem, m *prometheus.MetricVec) (map[string]string, float64) {
	val := e.extractGenericValue(flow, info)
	if val == nil {
		return nil, 0
	}
	floatVal, err := utils.ConvertToFloat64(val)
	if err != nil {
		e.errorsCounter.WithLabelValues("ValueConversionError", info.Name, info.ValueKey).Inc()
		return nil, 0
	}

	entryLabels, key := e.extractLabelsAndKey(flow, info)
	// Update entry for expiry mechanism (the entry itself is its own cleanup function)
	e.mCache.UpdateCacheEntry(key, func() { m.Delete(entryLabels) })
	return entryLabels, floatVal
}

func (e *EncodeProm) prepareAggHisto(flow config.GenericMap, info *api.PromMetricsItem, m *prometheus.MetricVec) (map[string]string, []float64) {
	val := e.extractGenericValue(flow, info)
	if val == nil {
		return nil, nil
	}
	values, ok := val.([]float64)
	if !ok {
		e.errorsCounter.WithLabelValues("HistoValueConversionError", info.Name, info.ValueKey).Inc()
		return nil, nil
	}

	entryLabels, key := e.extractLabelsAndKey(flow, info)
	// Update entry for expiry mechanism (the entry itself is its own cleanup function)
	e.mCache.UpdateCacheEntry(key, func() { m.Delete(entryLabels) })
	return entryLabels, values
}

func (e *EncodeProm) extractGenericValue(flow config.GenericMap, info *api.PromMetricsItem) interface{} {
	if info.Filter.Key != "" {
		if val, found := flow[info.Filter.Key]; found {
			sVal, ok := val.(string)
			if !ok {
				sVal = fmt.Sprint(val)
			}
			if sVal != info.Filter.Value {
				return nil
			}
		}
	}
	if info.ValueKey == "" {
		// No value key means it's a records / flows counter (1 flow = 1 increment), so just return 1
		return 1
	}
	val, found := flow[info.ValueKey]
	if !found {
		e.errorsCounter.WithLabelValues("RecordKeyMissing", info.Name, info.ValueKey).Inc()
		return nil
	}
	return val
}

func (e *EncodeProm) extractLabelsAndKey(flow config.GenericMap, info *api.PromMetricsItem) (map[string]string, string) {
	entryLabels := make(map[string]string, len(info.Labels))
	key := strings.Builder{}
	key.WriteString(info.Name)
	key.WriteRune('|')
	for _, t := range info.Labels {
		entryLabels[t] = ""
		if v, ok := flow[t]; ok {
			entryLabels[t] = fmt.Sprintf("%v", v)
		}
		key.WriteString(entryLabels[t])
		key.WriteRune('|')
	}
	return entryLabels, key.String()
}

// callback function from lru cleanup
func (e *EncodeProm) Cleanup(cleanupFunc interface{}) {
	cleanupFunc.(func())()
}

func (e *EncodeProm) cleanupExpiredEntriesLoop() {
	ticker := time.NewTicker(time.Duration(e.expiryTime) * time.Second)
	for {
		select {
		case <-e.exitChan:
			log.Debugf("exiting cleanupExpiredEntriesLoop because of signal")
			return
		case <-ticker.C:
			e.mCache.CleanupExpiredEntries(e.expiryTime, e)
		}
	}
}

// startServer listens for prometheus resource usage requests
func (e *EncodeProm) startServer() {
	log.Debugf("entering startServer")

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	var err error
	if e.tlsConfig != nil {
		err = e.server.ListenAndServeTLS(e.tlsConfig.CertPath, e.tlsConfig.KeyPath)
	} else {
		err = e.server.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		log.Errorf("error in http.ListenAndServe: %v", err)
		os.Exit(1)
	}
}

func (e *EncodeProm) closeServer(ctx context.Context) error {
	return e.server.Shutdown(ctx)
}

func NewEncodeProm(opMetrics *operational.Metrics, params config.StageParam) (Encoder, error) {
	cfg := api.PromEncode{}
	if params.Encode != nil && params.Encode.Prom != nil {
		cfg = *params.Encode.Prom
	}

	expiryTime := int64(cfg.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = defaultExpiryTime
	}
	log.Debugf("expiryTime = %d", expiryTime)

	counters := []counterInfo{}
	gauges := []gaugeInfo{}
	histos := []histoInfo{}
	aggHistos := []histoInfo{}

	for _, mInfo := range cfg.Metrics {
		fullMetricName := cfg.Prefix + mInfo.Name
		labels := mInfo.Labels
		log.Debugf("fullMetricName = %v", fullMetricName)
		log.Debugf("Labels = %v", labels)
		switch mInfo.Type {
		case api.PromEncodeOperationName("Counter"):
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(counter)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			counters = append(counters, counterInfo{
				counter: counter,
				info:    mInfo,
			})
		case api.PromEncodeOperationName("Gauge"):
			gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(gauge)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			gauges = append(gauges, gaugeInfo{
				gauge: gauge,
				info:  mInfo,
			})
		case api.PromEncodeOperationName("Histogram"):
			log.Debugf("buckets = %v", mInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: mInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			histos = append(histos, histoInfo{
				histo: hist,
				info:  mInfo,
			})
		case api.PromEncodeOperationName("AggHistogram"):
			log.Debugf("buckets = %v", mInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: mInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			aggHistos = append(aggHistos, histoInfo{
				histo: hist,
				info:  mInfo,
			})
		case "default":
			log.Errorf("invalid metric type = %v, skipping", mInfo.Type)
			continue
		}
	}

	log.Debugf("counters = %v", counters)
	log.Debugf("gauges = %v", gauges)
	log.Debugf("histos = %v", histos)
	log.Debugf("aggHistos = %v", aggHistos)

	addr := fmt.Sprintf(":%v", cfg.Port)
	log.Infof("startServer: addr = %s", addr)

	w := &EncodeProm{
		server: &http.Server{
			Addr: addr,
			// TLS clients must use TLS 1.2 or higher
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		tlsConfig:        cfg.TLS,
		counters:         counters,
		gauges:           gauges,
		histos:           histos,
		aggHistos:        aggHistos,
		expiryTime:       expiryTime,
		mCache:           utils.NewTimedCache(),
		exitChan:         utils.ExitChannel(),
		metricsProcessed: opMetrics.NewCounter(&metricsProcessed, params.Name),
		errorsCounter:    opMetrics.NewCounterVec(&encodePromErrors),
	}
	go w.startServer()
	go w.cleanupExpiredEntriesLoop()
	return w, nil
}
