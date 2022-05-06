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

package api

import "github.com/mariomac/pipes/pkg/graph/stage"

type PromEncode struct {
	stage.Instance
	Metrics    PromMetricsItems `yaml:"metrics" doc:"list of prometheus metric definitions, each includes:"`
	Port       int              `yaml:"port" doc:"port number to expose \"/metrics\" endpoint"`
	Prefix     string           `yaml:"prefix" doc:"prefix added to each metric name"`
	ExpiryTime int              `yaml:"expirytime" doc:"seconds of no-flow to wait before deleting prometheus data item"`
}

type PromEncodeOperationEnum struct {
	Gauge     string `yaml:"gauge" doc:"single numerical value that can arbitrarily go up and down"`
	Counter   string `yaml:"counter" doc:"monotonically increasing counter whose value can only increase"`
	Histogram string `yaml:"histogram" doc:"counts samples in configurable buckets"`
}

func PromEncodeOperationName(operation string) string {
	return GetEnumName(PromEncodeOperationEnum{}, operation)
}

type PromMetricsItem struct {
	Name     string            `yaml:"name" doc:"the metric name"`
	Type     string            `yaml:"type" enum:"PromEncodeOperationEnum" doc:"one of the following:"`
	Filter   PromMetricsFilter `yaml:"filter" doc:"the criterion to filter entries by"`
	ValueKey string            `yaml:"valuekey" doc:"entry key from which to resolve metric value"`
	Labels   []string          `yaml:"labels" doc:"labels to be associated with the metric"`
	Buckets  []float64         `yaml:"buckets" doc:"histogram buckets"`
}

type PromMetricsItems []PromMetricsItem

type PromMetricsFilter struct {
	Key   string `yaml:"key" doc:"the key to match and filter by"`
	Value string `yaml:"value" doc:"the value to match and filter by"`
}
