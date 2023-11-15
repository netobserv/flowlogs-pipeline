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

package opentelemetry

import (
	"context"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	flpTracerName     = "flp_tracer"
	flpEncodeSpanName = "flp_encode"
)

type EncodeOtlpTrace struct {
	cfg    api.EncodeOtlpTraces
	ctx    context.Context
	res    *resource.Resource
	tp     *sdktrace.TracerProvider
	tracer trace.Tracer
}

// Encode encodes a metric to be exported
func (e *EncodeOtlpTrace) Encode(metricRecord config.GenericMap) {
	log.Tracef("entering EncodeOtlpTrace. entry = %v", metricRecord)
	_, span := e.tracer.Start(e.ctx, flpEncodeSpanName)
	defer span.End()
	attributes := obtainAttributesFromEntry(metricRecord)
	span.SetAttributes(*attributes...)
}

func NewEncodeOtlpTraces(opMetrics *operational.Metrics, params config.StageParam) (encode.Encoder, error) {
	log.Tracef("entering NewEncodeOtlpTraces \n")
	cfg := api.EncodeOtlpTraces{}
	if params.Encode != nil && params.Encode.OtlpTraces != nil {
		cfg = *params.Encode.OtlpTraces
	}
	log.Debugf("NewEncodeOtlpTraces cfg = %v \n", cfg)

	ctx := context.Background()
	res := newResource()
	tracer := otel.Tracer(flpTracerName)

	tp, err := NewOtlpTracerProvider(ctx, params, res)
	if err != nil {
		return nil, err
	}

	w := &EncodeOtlpTrace{
		cfg:    cfg,
		ctx:    ctx,
		res:    res,
		tp:     tp,
		tracer: tracer,
	}
	return w, nil
}
