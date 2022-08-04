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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	operationalMetrics "github.com/netobserv/flowlogs-pipeline/pkg/operational/metrics"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const defaultExpiryTime = 120

type PromMetric struct {
	metricType  string
	promGauge   *prometheus.GaugeVec
	promCounter *prometheus.CounterVec
	promHist    *prometheus.HistogramVec
}

type keyValuePair struct {
	key   string
	value string
}

type metricInfo struct {
	input      string
	filter     keyValuePair
	labelNames []string
	PromMetric
}

type entrySignature struct {
	Name   string
	Labels map[string]string
}

type entryInfo struct {
	eInfo entrySignature
	PromMetric
}

type EncodeProm struct {
	port        string
	prefix      string
	metrics     map[string]metricInfo
	expiryTime  int64
	tlsConfig   api.PromTLSConf
	mCache      *utils.TimedCache
	exitChan    <-chan struct{}
	PrevRecords []config.GenericMap
}

var metricsProcessed = operationalMetrics.NewCounter(prometheus.CounterOpts{
	Name: "encode_prom_metrics_processed",
	Help: "Number of metrics processed",
})

// Encode encodes a metric before being stored
func (e *EncodeProm) Encode(metrics []config.GenericMap) {
	log.Debugf("entering EncodeProm Encode")
	out := make([]config.GenericMap, 0)
	for _, metric := range metrics {
		// TODO: We may need different handling for histograms
		metricsProcessed.Inc()
		metricOut := e.EncodeMetric(metric)
		out = append(out, metricOut...)
	}
	e.PrevRecords = out
	log.Debugf("out = %v", out)
	log.Debugf("cache = %v", e.mCache)
}

func (e *EncodeProm) EncodeMetric(metricRecord config.GenericMap) []config.GenericMap {
	log.Debugf("entering EncodeMetric. metricRecord = %v", metricRecord)
	out := make([]config.GenericMap, 0)
	for metricName, mInfo := range e.metrics {
		val, keyFound := metricRecord[mInfo.filter.key]
		shouldKeepRecord := keyFound && val == mInfo.filter.value
		if !shouldKeepRecord {
			continue
		}

		metricValue, ok := metricRecord[mInfo.input]
		if !ok {
			log.Errorf("field %v is missing", mInfo.input)
			continue
		}
		entryLabels := make(map[string]string, len(mInfo.labelNames))
		for _, t := range mInfo.labelNames {
			entryLabels[t] = fmt.Sprintf("%v", metricRecord[t])
		}
		entry := entryInfo{
			eInfo: entrySignature{
				Name:   e.prefix + metricName,
				Labels: entryLabels,
			},
		}
		key := generateCacheKey(&entry.eInfo)
		e.mCache.UpdateCacheEntry(key, entry)
		entry.PromMetric.metricType = mInfo.PromMetric.metricType
		// push the metric record to prometheus
		switch mInfo.PromMetric.metricType {
		case api.PromEncodeOperationName("Gauge"):
			metricValueFloat, err := utils.ConvertToFloat64(metricValue)
			if err != nil {
				log.Errorf("value cannot be converted to float64. err: %v, metric: %v, key: %v, value: %v", err, metricName, mInfo.input, metricValue)
				continue
			}
			mInfo.promGauge.With(entryLabels).Set(metricValueFloat)
			entry.PromMetric.promGauge = mInfo.promGauge
		case api.PromEncodeOperationName("Counter"):
			metricValueFloat, err := utils.ConvertToFloat64(metricValue)
			if err != nil {
				log.Errorf("value cannot be converted to float64. err: %v, metric: %v, key: %v, value: %v", err, metricName, mInfo.input, metricValue)
				continue
			}
			mInfo.promCounter.With(entryLabels).Add(metricValueFloat)
			entry.PromMetric.promCounter = mInfo.promCounter
		case api.PromEncodeOperationName("Histogram"):
			metricValueSlice, ok := metricValue.([]float64)
			if !ok {
				log.Errorf("value is not []float64. metric: %v, key: %v, value: %v", metricName, mInfo.input, metricValue)
				continue
			}
			for _, v := range metricValueSlice {
				mInfo.promHist.With(entryLabels).Observe(v)
			}
			entry.PromMetric.promHist = mInfo.promHist
		}

		entryMap := map[string]interface{}{
			// TODO: change to lower case
			"Name":   e.prefix + metricName,
			"Labels": entryLabels,
			"value":  metricValue,
		}
		out = append(out, entryMap)
	}
	return out
}

func generateCacheKey(sig *entrySignature) string {
	eInfoString := fmt.Sprintf("%s%v", sig.Name, sig.Labels)
	log.Debugf("generateCacheKey: eInfoString = %s", eInfoString)
	return eInfoString
}

// callback function from lru cleanup
func (e *EncodeProm) Cleanup(sourceEntry interface{}) {
	entry := sourceEntry.(entryInfo)
	// clean up the entry
	log.Debugf("deleting %v", entry)
	switch entry.PromMetric.metricType {
	case api.PromEncodeOperationName("Gauge"):
		entry.PromMetric.promGauge.Delete(entry.eInfo.Labels)
	case api.PromEncodeOperationName("Counter"):
		entry.PromMetric.promCounter.Delete(entry.eInfo.Labels)
	case api.PromEncodeOperationName("Histogram"):
		entry.PromMetric.promHist.Delete(entry.eInfo.Labels)
	}
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

// startPrometheusInterface listens for prometheus resource usage requests
func startPrometheusInterface(w *EncodeProm) {
	log.Debugf("entering startPrometheusInterface")
	log.Infof("startPrometheusInterface: port num = %s", w.port)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	var err error
	if w.tlsConfig.Enable {
		err = http.ListenAndServeTLS(w.port, w.tlsConfig.CertFile, w.tlsConfig.KeyFile, nil)
	} else {
		err = http.ListenAndServe(w.port, nil)
	}
	if err != nil {
		log.Errorf("error in http.ListenAndServe: %v", err)
		os.Exit(1)
	}
}

func NewEncodeProm(params config.StageParam) (Encoder, error) {
	jsonEncodeProm := api.PromEncode{}
	if params.Encode != nil && params.Encode.Prom != nil {
		jsonEncodeProm = *params.Encode.Prom
	}

	portNum := jsonEncodeProm.Port
	promPrefix := jsonEncodeProm.Prefix
	expiryTime := int64(jsonEncodeProm.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = defaultExpiryTime
	}
	log.Debugf("expiryTime = %d", expiryTime)

	metrics := make(map[string]metricInfo)
	for _, mInfo := range jsonEncodeProm.Metrics {
		var pMetric PromMetric
		fullMetricName := promPrefix + mInfo.Name
		labels := mInfo.Labels
		log.Debugf("fullMetricName = %v", fullMetricName)
		log.Debugf("Labels = %v", labels)
		pMetric.metricType = mInfo.Type
		switch mInfo.Type {
		case api.PromEncodeOperationName("Counter"):
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(counter)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			pMetric.promCounter = counter
		case api.PromEncodeOperationName("Gauge"):
			gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(gauge)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			pMetric.promGauge = gauge
		case api.PromEncodeOperationName("Histogram"):
			log.Debugf("buckets = %v", mInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: mInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			pMetric.promHist = hist
		case "default":
			log.Errorf("invalid metric type = %v, skipping", mInfo.Type)
			continue
		}
		metrics[mInfo.Name] = metricInfo{
			input: mInfo.ValueKey,
			filter: keyValuePair{
				key:   mInfo.Filter.Key,
				value: mInfo.Filter.Value,
			},
			labelNames: labels,
			PromMetric: pMetric,
		}
	}

	log.Debugf("metrics = %v", metrics)
	w := &EncodeProm{
		port:        fmt.Sprintf(":%v", portNum),
		prefix:      promPrefix,
		metrics:     metrics,
		expiryTime:  expiryTime,
		mCache:      utils.NewTimedCache(),
		exitChan:    utils.ExitChannel(),
		PrevRecords: make([]config.GenericMap, 0),
		tlsConfig:   jsonEncodeProm.TLS,
	}
	go startPrometheusInterface(w)
	go w.cleanupExpiredEntriesLoop()
	return w, nil
}
