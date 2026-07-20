/*
 * Copyright (C) 2026 NetObserv Authors.
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

package flowbuffer

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/server"
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxEntries   = 50000
	defaultListenAddr   = ":9200"
	defaultQueryTimeout = 2 * time.Second
)

var wlog = logrus.WithField("component", "write.FlowBuffer")

// Writer is a pipeline write stage that stores enriched flows in a ring buffer
// and exposes local + cluster query HTTP APIs.
type Writer struct {
	ring   *Ring
	server *http.Server
}

// Write stores a flow in the ring buffer.
func (w *Writer) Write(in config.GenericMap) {
	w.ring.Insert(in)
}

// Ring returns the underlying buffer (for tests).
func (w *Writer) Ring() *Ring {
	return w.ring
}

// NewWriter creates a flowBuffer writer and starts the query HTTP server.
func NewWriter(params config.StageParam) (*Writer, error) {
	cfg := api.WriteFlowBuffer{}
	if params.Write != nil && params.Write.FlowBuffer != nil {
		cfg = *params.Write.FlowBuffer
	}

	maxEntries := cfg.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	listenAddr := cfg.QueryListenAddress
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}
	timeout := cfg.QueryTimeout.Duration
	if timeout <= 0 {
		timeout = defaultQueryTimeout
	}

	ring := NewRing(maxEntries)
	peers, err := buildPeerLister(cfg, listenAddr)
	if err != nil {
		return nil, err
	}

	qs := NewServer(ring, peers, timeout, listenAddr)
	httpServer := server.Default(&http.Server{
		Addr:    listenAddr,
		Handler: qs.Handler(),
	})
	// Query responses can be large; relax write timeout relative to query timeout.
	httpServer.WriteTimeout = timeout + 5*time.Second
	httpServer.ReadTimeout = timeout + 5*time.Second

	w := &Writer{ring: ring, server: httpServer}
	go w.serve()
	go w.shutdownOnExit()
	wlog.Infof("flowBuffer started: maxEntries=%d listen=%s", maxEntries, listenAddr)
	return w, nil
}

func buildPeerLister(cfg api.WriteFlowBuffer, listenAddr string) (PeerLister, error) {
	if len(cfg.PeerURLs) > 0 {
		return StaticPeers{URLs: cfg.PeerURLs}, nil
	}
	if cfg.ServiceName == "" {
		// Single-pod / no discovery: cluster queries only hit the local buffer.
		return nil, nil
	}
	port := cfg.PeerPort
	if port == 0 {
		_, portStr, err := net.SplitHostPort(listenAddr)
		if err == nil {
			if p, err2 := strconv.Atoi(portStr); err2 == nil {
				port = p
			}
		}
	}
	if port == 0 {
		port = 9200
	}
	return NewEndpointSlicePeers(cfg.Namespace, cfg.ServiceName, port, cfg.KubeConfigPath)
}

func (w *Writer) serve() {
	if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		wlog.Errorf("flowBuffer query server error: %v", err)
	}
}

func (w *Writer) shutdownOnExit() {
	<-utils.ExitChannel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.server.Shutdown(ctx); err != nil {
		wlog.Warnf("flowBuffer server shutdown: %v", err)
	}
}

// ListenAddr returns the configured listen address (tests).
func (w *Writer) ListenAddr() string {
	if w.server == nil {
		return ""
	}
	return w.server.Addr
}
