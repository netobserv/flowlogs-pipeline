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
	"encoding/json"
	"net/http"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// HealthServer provides HTTP health and readiness endpoints
type HealthServer struct {
	server   *http.Server
	ready    atomic.Bool
	isLeader atomic.Bool
}

// HealthStatus represents the health check response
type HealthStatus struct {
	Status   string `json:"status"`
	IsLeader bool   `json:"isLeader"`
	Ready    bool   `json:"ready"`
}

// NewHealthServer creates a new health server listening on the specified port
func NewHealthServer(port int) *HealthServer {
	hs := &HealthServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", hs.healthHandler)
	mux.HandleFunc("/ready", hs.readyHandler)
	mux.HandleFunc("/status", hs.statusHandler)

	hs.server = &http.Server{
		Addr:    formatAddress(port),
		Handler: mux,
	}

	return hs
}

// Start starts the health server in a goroutine
func (hs *HealthServer) Start() error {
	log.WithField("address", hs.server.Addr).Info("Starting health server")

	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("Health server error")
		}
	}()

	return nil
}

// Stop stops the health server gracefully
func (hs *HealthServer) Stop() error {
	log.Info("Stopping health server")
	return hs.server.Close()
}

// SetReady marks the server as ready
func (hs *HealthServer) SetReady(ready bool) {
	hs.ready.Store(ready)
}

// SetLeader marks this instance as leader or follower
func (hs *HealthServer) SetLeader(isLeader bool) {
	hs.isLeader.Store(isLeader)
	if isLeader {
		log.Info("Became leader")
	} else {
		log.Info("Lost leadership")
	}
}

// healthHandler always returns 200 OK if the process is running
func (hs *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler returns 200 OK only if the service is ready
func (hs *HealthServer) readyHandler(w http.ResponseWriter, r *http.Request) {
	if hs.ready.Load() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
	}
}

// statusHandler returns detailed status information as JSON
func (hs *HealthServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:   "OK",
		IsLeader: hs.isLeader.Load(),
		Ready:    hs.ready.Load(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}
