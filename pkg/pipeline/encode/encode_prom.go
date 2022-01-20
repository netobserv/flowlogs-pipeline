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
	"encoding/json"
	"fmt"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
)

// TODO when do we clean up old entries?

type gaugeInfo struct {
	input     string
	tags      []string
	promGauge *prometheus.GaugeVec
}

type counterInfo struct {
	input       string
	tags        []string
	promCounter *prometheus.CounterVec
}

type histInfo struct {
	input    string
	tags     []string
	promHist *prometheus.HistogramVec
}

type counterEntryInfo struct {
	counterName  string
	counterValue float64
	labels       map[string]string
}

type gaugeEntryInfo struct {
	gaugeName  string
	gaugeValue float64
	labels     map[string]string
}

type encodeProm struct {
	port       string
	prefix     string
	counters   map[string]counterInfo
	gauges     map[string]gaugeInfo
	histograms map[string]histInfo
}

func (e *encodeProm) EncodeCounter(metric config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for counterName, counterInfo := range e.counters {
		counterValue, ok := metric[counterInfo.input]
		if !ok {
			log.Debugf("field %v is missing", counterName)
			continue
		}
		counterValueString := fmt.Sprintf("%v", counterValue)
		valueFloat, err := strconv.ParseFloat(counterValueString, 64)
		if err != nil {
			log.Debugf("field cannot be converted to float: %v, %s", counterValue, counterValueString)
			continue
		}
		entryLabels := make(map[string]string, len(counterInfo.tags))
		for _, t := range counterInfo.tags {
			entryLabels[t] = fmt.Sprintf("%v", metric[t])
		}
		entry := counterEntryInfo{
			counterName:  e.prefix + counterName,
			counterValue: valueFloat,
			labels:       entryLabels,
		}
		out = append(out, entry)
		// push the metric to prometheus
		// TODO - fix the types here
		if counterInfo.promCounter != nil {
			counterInfo.promCounter.With(entryLabels).Add(valueFloat)
		}
	}

	return out
}
func (e *encodeProm) EncodeGauge(metric config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for gaugeName, gaugeInfo := range e.gauges {
		gaugeValue, ok := metric[gaugeInfo.input]
		if !ok {
			log.Debugf("field %v is missing", gaugeName)
			continue
		}
		gaugeValueString := fmt.Sprintf("%v", gaugeValue)
		valueFloat, err := strconv.ParseFloat(gaugeValueString, 64)
		if err != nil {
			log.Debugf("field cannot be converted to float: %v, %s", gaugeValue, gaugeValueString)
			continue
		}
		entryLabels := make(map[string]string, len(gaugeInfo.tags))
		for _, t := range gaugeInfo.tags {
			entryLabels[t] = fmt.Sprintf("%v", metric[t])
		}
		entry := gaugeEntryInfo{
			gaugeName:  e.prefix + gaugeName,
			gaugeValue: valueFloat,
			labels:     entryLabels,
		}
		out = append(out, entry)
		// push the metric to prometheus
		// TODO - fix the types here
		if gaugeInfo.promGauge != nil {
			gaugeInfo.promGauge.With(entryLabels).Set(valueFloat)
		}
	}

	return out
}

// Encode encodes a flow before being stored
func (e *encodeProm) Encode(metrics []config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for _, metric := range metrics {
		gaugeOut := e.EncodeGauge(metric)
		out = append(out, gaugeOut...)
		counterOut := e.EncodeCounter(metric)
		out = append(out, counterOut...)
	}
	return out
}

// startPrometheusInterface listens for prometheus resource usage requests
func startPrometheusInterface(w *encodeProm) {
	log.Debugf("entering startPrometheusInterface")
	log.Infof("startPrometheusInterface: port num = %s", w.port)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	err := http.ListenAndServe(w.port, nil)
	if err != nil {
		log.Errorf("error in http.ListenAndServe: %v", err)
		os.Exit(1)
	}
}

func NewEncodeProm() (Encoder, error) {
	encodePromString := config.Opt.PipeLine.Encode.Prom
	log.Debugf("promEncodeString = %s", encodePromString)
	var jsonEncodeProm api.PromEncode
	err := json.Unmarshal([]byte(encodePromString), &jsonEncodeProm)
	if err != nil {
		return nil, err
	}

	portNum := jsonEncodeProm.Port
	promPrefix := jsonEncodeProm.Prefix

	counters := make(map[string]counterInfo)
	gauges := make(map[string]gaugeInfo)
	histograms := make(map[string]histInfo)
	for _, metricInfo := range jsonEncodeProm.Metrics {
		fullMetricName := promPrefix + metricInfo.Name
		labels := metricInfo.Labels
		log.Debugf("fullMetricName = %v", fullMetricName)
		log.Debugf("labels = %v", labels)
		switch metricInfo.Type {
		case api.PromEncodeOperationName("Counter"):
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(counter)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			counters[metricInfo.Name] = counterInfo{metricInfo.ValueKey, labels, counter}
		case api.PromEncodeOperationName("Gauge"):
			gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(gauge)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			gauges[metricInfo.Name] = gaugeInfo{metricInfo.ValueKey, labels, gauge}
		case api.PromEncodeOperationName("Histogram"):
			log.Debugf("buckets = %v", metricInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: metricInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			histograms[metricInfo.Name] = histInfo{metricInfo.ValueKey, labels, hist}
		case "default":
			log.Errorf("invalid metric type = %v, skipping", metricInfo.Type)
		}
	}

	w := &encodeProm{
		port:       fmt.Sprintf(":%v", portNum),
		prefix:     promPrefix,
		counters:   counters,
		gauges:     gauges,
		histograms: histograms,
	}
	go startPrometheusInterface(w)
	return w, nil
}
