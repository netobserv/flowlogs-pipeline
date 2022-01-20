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
	"container/list"
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
	"sync"
	"time"
)

const defaultExpiryTime = 120

type genericMetricInfo struct {
	input       string
	tags        []string
	metricType  string
	promGauge   *prometheus.GaugeVec
	promCounter *prometheus.CounterVec
	promHist    *prometheus.HistogramVec
}

type entrySignature struct {
	Name   string
	Labels map[string]string
}

type entryInfo struct {
	eInfo entrySignature
	value float64
}

type metricCacheEntry struct {
	label       prometheus.Labels
	timeStamp   int64
	e           *list.Element
	key         string
	metricType  string
	promGauge   *prometheus.GaugeVec
	promCounter *prometheus.CounterVec
	promHist    *prometheus.HistogramVec
}

type metricCache map[string]*metricCacheEntry

type encodeProm struct {
	mu         sync.Mutex
	port       string
	prefix     string
	counters   map[string]genericMetricInfo
	gauges     map[string]genericMetricInfo
	histograms map[string]genericMetricInfo
	expiryTime int64
	mList      *list.List
	mCache     metricCache
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
		entry := entryInfo{
			eInfo: entrySignature{
				Name:   e.prefix + counterName,
				Labels: entryLabels,
			},
			value: valueFloat,
		}
		out = append(out, entry)
		// push the metric to prometheus
		if counterInfo.promCounter != nil {
			counterInfo.promCounter.With(entryLabels).Add(valueFloat)
		}
		cEntry := e.saveEntryInCache(entry, entryLabels)
		cEntry.metricType = api.PromEncodeOperationName("Counter")
		cEntry.promCounter = counterInfo.promCounter
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
		entry := entryInfo{
			eInfo: entrySignature{
				Name:   e.prefix + gaugeName,
				Labels: entryLabels,
			},
			value: valueFloat,
		}
		out = append(out, entry)
		// push the metric to prometheus
		if gaugeInfo.promGauge != nil {
			gaugeInfo.promGauge.With(entryLabels).Set(valueFloat)
		}

		cEntry := e.saveEntryInCache(entry, entryLabels)
		cEntry.metricType = api.PromEncodeOperationName("Gauge")
		cEntry.promGauge = gaugeInfo.promGauge
	}
	return out
}

func (e *encodeProm) saveEntryInCache(entry entryInfo, entryLabels map[string]string) *metricCacheEntry {
	// save item in cache; use eInfo as key to the cache
	var cEntry *metricCacheEntry
	secs := time.Now().Unix()
	eInfoBytes, _ := json.Marshal(&entry.eInfo)
	eInfoString := string(eInfoBytes)
	cEntry, ok := e.mCache[eInfoString]
	if ok {
		// item already exists in cache; update the element and move to end of list
		cEntry.timeStamp = secs
		// move to end of list
		e.mList.MoveToBack(cEntry.e)
	} else {
		// create new entry for cache
		cEntry = &metricCacheEntry{
			label:     entryLabels,
			timeStamp: secs,
			key:       eInfoString,
		}
		// place at end of list
		log.Debugf("adding entry = %v", cEntry)
		cEntry.e = e.mList.PushBack(cEntry)
		e.mCache[eInfoString] = cEntry
		log.Debugf("mlist = %v", e.mList)
	}
	return cEntry
}

// Encode encodes a flow before being stored
func (e *encodeProm) Encode(metrics []config.GenericMap) []interface{} {
	log.Debugf("entering encodeProm Encode")
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]interface{}, 0)
	for _, metric := range metrics {
		gaugeOut := e.EncodeGauge(metric)
		out = append(out, gaugeOut...)
		counterOut := e.EncodeCounter(metric)
		out = append(out, counterOut...)
	}
	log.Debugf("cache = %v", e.mCache)
	log.Debugf("list = %v", e.mList)
	return out
}

func (e *encodeProm) cleanupExpiredEntriesLoop() {
	for {
		e.cleanupExpiredEntries()
		time.Sleep(time.Duration(e.expiryTime) * time.Second)
	}
}

// cleanupExpiredEntries - any entry that has expired should be removed from the prometheus reporting and cache
func (e *encodeProm) cleanupExpiredEntries() {
	log.Debugf("entering cleanupExpiredEntries")
	e.mu.Lock()
	defer e.mu.Unlock()
	log.Debugf("cache = %v", e.mCache)
	log.Debugf("list = %v", e.mList)
	secs := time.Now().Unix()
	expireTime := secs - e.expiryTime
	// go through the list until we reach recently used connections
	for {
		entry := e.mList.Front()
		if entry == nil {
			return
		}
		c := entry.Value.(*metricCacheEntry)
		log.Debugf("timeStamp = %d, expireTime = %d", c.timeStamp, expireTime)
		log.Debugf("c = %v", c)
		if c.timeStamp > expireTime {
			// no more expired items
			return
		}

		// clean up the entry
		log.Debugf("secs = %d, deleting %s", secs, c.label)
		switch c.metricType {
		case api.PromEncodeOperationName("Gauge"):
			c.promGauge.Delete(c.label)
		case api.PromEncodeOperationName("Counter"):
			c.promCounter.Delete(c.label)
		case api.PromEncodeOperationName("Histogram"):
			c.promHist.Delete(c.label)
		}
		delete(e.mCache, c.key)
		e.mList.Remove(entry)
	}
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
	expiryTime := int64(jsonEncodeProm.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = defaultExpiryTime
	}
	log.Debugf("expiryTime = %d", expiryTime)

	counters := make(map[string]genericMetricInfo)
	gauges := make(map[string]genericMetricInfo)
	histograms := make(map[string]genericMetricInfo)
	for _, metricInfo := range jsonEncodeProm.Metrics {
		fullMetricName := promPrefix + metricInfo.Name
		labels := metricInfo.Labels
		log.Debugf("fullMetricName = %v", fullMetricName)
		log.Debugf("Labels = %v", labels)
		switch metricInfo.Type {
		case api.PromEncodeOperationName("Counter"):
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(counter)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			counters[metricInfo.Name] = genericMetricInfo{
				input:       metricInfo.ValueKey,
				tags:        labels,
				metricType:  api.PromEncodeOperationName("Counter"),
				promCounter: counter,
			}
		case api.PromEncodeOperationName("Gauge"):
			gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(gauge)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			gauges[metricInfo.Name] = genericMetricInfo{
				input:      metricInfo.ValueKey,
				tags:       labels,
				metricType: api.PromEncodeOperationName("Gauge"),
				promGauge:  gauge,
			}
		case api.PromEncodeOperationName("Histogram"):
			log.Debugf("buckets = %v", metricInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: metricInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			histograms[metricInfo.Name] = genericMetricInfo{
				input:      metricInfo.ValueKey,
				tags:       labels,
				metricType: api.PromEncodeOperationName("Histogram"),
				promHist:   hist,
			}
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
		expiryTime: expiryTime,
		mList:      list.New(),
		mCache:     make(metricCache),
	}
	go startPrometheusInterface(w)
	go w.cleanupExpiredEntriesLoop()
	return w, nil
}
