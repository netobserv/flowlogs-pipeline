/*
 * Copyright (C) 2022 IBM, Inc.
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

package health

import (
	"net"
	"net/http"
	"time"

	"github.com/heptiolabs/healthcheck"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	log "github.com/sirupsen/logrus"
)

const defaultServerHost = "0.0.0.0"

type Server struct {
	handler healthcheck.Handler
	address string
}

func (hs *Server) Serve() {
	for {
		err := http.ListenAndServe(hs.address, hs.handler)
		log.Errorf("http.ListenAndServe error %v", err)
		time.Sleep(60 * time.Second)
	}
}

func NewHealthServer(opts *config.Options, pipeline *pipeline.Pipeline) *Server {

	handler := healthcheck.NewHandler()
	address := net.JoinHostPort(defaultServerHost, opts.Health.Port)

	handler.AddLivenessCheck("PipelineCheck", pipeline.IsAlive())
	handler.AddReadinessCheck("PipelineCheck", pipeline.IsReady())

	server := &Server{
		handler: handler,
		address: address,
	}

	go server.Serve()

	return server
}
