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

package test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/golang/snappy"
	"github.com/netobserv/loki-client-go/pkg/logproto"
	log "github.com/sirupsen/logrus"
)

// FakeLokiHandler is a fake loki HTTP service that decodes the snappy/protobuf messages
// and forwards them for later assertions
func FakeLokiHandler(flowsData chan<- map[string]interface{}) http.HandlerFunc {
	hlog := log.WithField("component", "LokiHandler")
	return func(rw http.ResponseWriter, req *http.Request) {
		hlog.WithFields(log.Fields{
			"method": req.Method,
			"url":    req.URL,
			"header": req.Header,
		}).Info("new request")
		if req.Method != http.MethodPost && req.Method != http.MethodPut {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			hlog.WithError(err).Error("can't read request body")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		decodedBody, err := snappy.Decode([]byte{}, body)
		if err != nil {
			hlog.WithError(err).Error("can't decode snappy body")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		pr := logproto.PushRequest{}
		if err := pr.Unmarshal(decodedBody); err != nil {
			hlog.WithError(err).Error("can't decode protobuf body")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, stream := range pr.Streams {
			for _, entry := range stream.Entries {
				flowData := map[string]interface{}{}
				if err := json.Unmarshal([]byte(entry.Line), &flowData); err != nil {
					hlog.WithError(err).Error("expecting JSON line")
					rw.WriteHeader(http.StatusBadRequest)
					return
				}
				// TODO: decorate the flow map with extra metadata from the stream entry
				flowsData <- flowData
			}
		}
		rw.WriteHeader(http.StatusOK)
	}
}
