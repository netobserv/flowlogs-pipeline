/*
 * Copyright (C) 2023 IBM, Inc.
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

package utils

import (
	"fmt"
	"net/http"
	"os"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// StartPromServer listens for prometheus resource usage requests
func StartPromServer(cfg *config.MetricsSettings, server *http.Server) {
	logrus.Debugf("entering StartPromServer")

	// if value of address is empty, then by default it will take 0.0.0.0
	server.Addr = fmt.Sprintf("%s:%v", cfg.Address, cfg.Port)
	log.Infof("Prometheus server: addr = %s", server.Addr)
	tlsConfig, err := cfg.TLS.Build()
	if err != nil {
		logrus.Errorf("error getting TLS configuration: %v", err)
		if !cfg.NoPanic {
			os.Exit(1)
		}
	}
	server.TLSConfig = tlsConfig

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	if tlsConfig != nil {
		err = server.ListenAndServeTLS(cfg.TLS.CertPath, cfg.TLS.KeyPath)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		logrus.Errorf("error in http.ListenAndServe: %v", err)
		if !cfg.NoPanic {
			os.Exit(1)
		}
	}
}
