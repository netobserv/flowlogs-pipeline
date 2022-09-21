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

type AggregateBy []string
type AggregateOperation string

type AggregateDefinition struct {
	Name          string             `yaml:"name,omitempty" json:"name,omitempty" doc:"description of aggregation result"`
	GroupByKeys   AggregateBy        `yaml:"groupByKeys,omitempty" json:"groupByKeys,omitempty" doc:"list of fields on which to aggregate"`
	OperationType AggregateOperation `yaml:"operationType,omitempty" json:"operationType,omitempty" doc:"sum, min, max, avg or raw_values"`
	OperationKey  string             `yaml:"operationKey,omitempty" json:"operationKey,omitempty" doc:"internal field on which to perform the operation"`
}
