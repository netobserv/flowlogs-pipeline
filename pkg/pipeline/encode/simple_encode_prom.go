package encode

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	operationalMetrics "github.com/netobserv/flowlogs-pipeline/pkg/operational/metrics"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type gaugeInfo struct {
	gauge *prometheus.GaugeVec
	info  *api.SimplePromMetricsItem
}

type counterInfo struct {
	counter *prometheus.CounterVec
	info    *api.SimplePromMetricsItem
}

type histoInfo struct {
	histo *prometheus.HistogramVec
	info  *api.SimplePromMetricsItem
}
type SimpleEncodeProm struct {
	gauges     []gaugeInfo
	counters   []counterInfo
	histos     []histoInfo
	expiryTime int64
	mCache     *utils.TimedCache
	exitChan   <-chan struct{}
}

var errorsCounter = operationalMetrics.NewCounterVec(prometheus.CounterOpts{
	Name: "encode_prom_errors",
	Help: "Total errors during metrics generation",
}, []string{"error", "metric", "key"})

// Encode encodes a metric before being stored
func (e *SimpleEncodeProm) Encode(flows []config.GenericMap) {
	log.Debugf("entering SimpleEncodeProm Encode")
	for _, flow := range flows {
		e.FlowToMetrics(flow)
	}
}

func (e *SimpleEncodeProm) FlowToMetrics(flow config.GenericMap) {
	log.Debugf("entering SimpleEncodeProm.FlowToMetrics metricRecord = %v", flow)

	// Process counters
	for _, mInfo := range e.counters {
		labels, value := e.prepareMetric(flow, mInfo.info, mInfo.counter.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.counter.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Add(value)
		metricsProcessed.Inc()
	}

	// Process gauges
	for _, mInfo := range e.gauges {
		labels, value := e.prepareMetric(flow, mInfo.info, mInfo.gauge.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.gauge.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Set(value)
		metricsProcessed.Inc()
	}

	// Process histograms
	for _, mInfo := range e.histos {
		labels, value := e.prepareMetric(flow, mInfo.info, mInfo.histo.MetricVec)
		if labels == nil {
			continue
		}
		m, err := mInfo.histo.GetMetricWith(labels)
		if err != nil {
			log.Errorf("labels registering error on %s: %v", mInfo.info.Name, err)
			errorsCounter.WithLabelValues("LabelsRegisteringError", mInfo.info.Name, "").Inc()
			continue
		}
		m.Observe(value)
		metricsProcessed.Inc()
	}
}

func (e *SimpleEncodeProm) prepareMetric(flow config.GenericMap, info *api.SimplePromMetricsItem, m *prometheus.MetricVec) (map[string]string, float64) {
	val, found := flow[info.RecordKey]
	if !found {
		errorsCounter.WithLabelValues("RecordKeyMissing", info.Name, info.RecordKey).Inc()
		return nil, 0
	}
	floatVal, err := utils.ConvertToFloat64(val)
	if err != nil {
		errorsCounter.WithLabelValues("ValueConversionError", info.Name, info.RecordKey).Inc()
		return nil, 0
	}

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
	// Update entry for expiry mechanism (the entry itself is its own cleanup function)
	e.mCache.UpdateCacheEntry(key.String(), func() { m.Delete(entryLabels) })
	return entryLabels, floatVal
}

// startServer listens for prometheus resource usage requests
func (e *SimpleEncodeProm) startServer(port int) {
	log.Debugf("entering startServer")
	addr := fmt.Sprintf(":%v", port)
	log.Infof("startServer: addr = %s", addr)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Errorf("error in http.ListenAndServe: %v", err)
		os.Exit(1)
	}
}

// callback function from lru cleanup
func (e *SimpleEncodeProm) Cleanup(cleanupFunc interface{}) {
	cleanupFunc.(func())()
}

func (e *SimpleEncodeProm) cleanupExpiredEntriesLoop() {
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

func NewEncodeSimpleProm(params config.StageParam) (Encoder, error) {
	config := api.SimplePromEncode{}
	if params.Encode != nil && params.Encode.SimpleProm != nil {
		config = *params.Encode.SimpleProm
	}

	expiryTime := int64(config.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = defaultExpiryTime
	}
	log.Debugf("expiryTime = %d", expiryTime)

	counters := []counterInfo{}
	gauges := []gaugeInfo{}
	histos := []histoInfo{}

	for i := range config.Metrics {
		mInfo := config.Metrics[i]
		fullMetricName := config.Prefix + mInfo.Name
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
				info:    &mInfo,
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
				info:  &mInfo,
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
				info:  &mInfo,
			})
		case "default":
			log.Errorf("invalid metric type = %v, skipping", mInfo.Type)
			continue
		}
	}

	log.Debugf("counters = %v", counters)
	log.Debugf("gauges = %v", gauges)
	log.Debugf("histos = %v", histos)
	w := &SimpleEncodeProm{
		counters:   counters,
		gauges:     gauges,
		histos:     histos,
		expiryTime: expiryTime,
		mCache:     utils.NewTimedCache(),
		exitChan:   utils.ExitChannel(),
	}
	go w.startServer(config.Port)
	go w.cleanupExpiredEntriesLoop()
	return w, nil
}
