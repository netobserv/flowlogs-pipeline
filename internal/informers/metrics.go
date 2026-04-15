/*
 * Copyright (C) 2024 Red Hat, Inc.
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

package informers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	// InformersMetrics holds all Prometheus metrics for flp-informers
	InformersMetrics *Metrics
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	IsLeader                prometheus.Gauge
	ConnectedProcessors     prometheus.Gauge
	CacheUpdatesTotal       *prometheus.CounterVec
	CacheSnapshotsSentTotal prometheus.Counter
	DiscoveryErrors         prometheus.Counter
}

// InitMetrics initializes all Prometheus metrics
func InitMetrics() {
	InformersMetrics = &Metrics{
		IsLeader: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flp_informers_is_leader",
			Help: "1 if this instance is the current leader, 0 otherwise",
		}),
		ConnectedProcessors: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flp_informers_connected_processors",
			Help: "Number of FLP processors currently connected",
		}),
		CacheUpdatesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flp_informers_cache_updates_total",
				Help: "Total number of cache updates sent to processors",
			},
			[]string{"operation"}, // ADD, UPDATE, DELETE, SNAPSHOT
		),
		CacheSnapshotsSentTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flp_informers_snapshots_sent_total",
			Help: "Total number of full snapshots sent to processors",
		}),
		DiscoveryErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flp_informers_discovery_errors_total",
			Help: "Total number of processor discovery errors",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		InformersMetrics.IsLeader,
		InformersMetrics.ConnectedProcessors,
		InformersMetrics.CacheUpdatesTotal,
		InformersMetrics.CacheSnapshotsSentTotal,
		InformersMetrics.DiscoveryErrors,
	)
}

// MetricsServer provides HTTP endpoint for Prometheus metrics
type MetricsServer struct {
	server *http.Server
}

// NewMetricsServer creates a new metrics server listening on the specified port
func NewMetricsServer(port int) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &MetricsServer{
		server: &http.Server{
			Addr:    formatAddress(port),
			Handler: mux,
		},
	}
}

// Start starts the metrics server in a goroutine
func (ms *MetricsServer) Start() error {
	log.WithField("address", ms.server.Addr).Info("Starting metrics server")

	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("Metrics server error")
		}
	}()

	return nil
}

// Stop stops the metrics server gracefully
func (ms *MetricsServer) Stop() error {
	log.Info("Stopping metrics server")
	return ms.server.Close()
}
