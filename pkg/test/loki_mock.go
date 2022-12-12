package test

import (
	"encoding/json"
	"io"
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
		body, err := io.ReadAll(req.Body)
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
				go func() {
					flowsData <- flowData
				}()
			}
		}
		rw.WriteHeader(http.StatusOK)
	}
}
