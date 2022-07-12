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

package operationalMetrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metricDefinition struct {
	Name string
	Help string
	Type string
}

var metricsOpts []metricDefinition

func NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	metricsOpts = append(metricsOpts, metricDefinition{
		Name: opts.Name,
		Help: opts.Help,
		Type: "counter",
	})
	return promauto.NewCounter(opts)
}

func NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	metricsOpts = append(metricsOpts, metricDefinition{
		Name: opts.Name,
		Help: opts.Help,
		Type: "counter",
	})
	return promauto.NewCounterVec(opts, labelNames)
}

func NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	metricsOpts = append(metricsOpts, metricDefinition{
		Name: opts.Name,
		Help: opts.Help,
		Type: "gauge",
	})
	return promauto.NewGauge(opts)
}

func GetDocumentation() string {
	doc := ""
	for _, opts := range metricsOpts {
		doc += fmt.Sprintf(
			`
### %s
| **Name** | %s | 
|:---|:---|
| **Description** | %s | 
| **Type** | %s | 

`,
			opts.Name,
			opts.Name,
			opts.Help,
			opts.Type,
		)
	}

	return doc
}
