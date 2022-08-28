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

type FilterOperation string

type ExtractTimebased struct {
	Rules []TimebasedFilterRule `yaml:"rules,omitempty" json:"rules,omitempty" doc:"list of filter rules, each includes:"`
}

type TimebasedFilterRule struct {
	Name         string          `yaml:"name,omitempty" json:"name,omitempty" doc:"description of filter result"`
	RecordKey    string          `yaml:"recordKey,omitempty" json:"recordKey,omitempty" doc:"internal field to index TopK/BotK "`
	Operation    FilterOperation `yaml:"operation,omitempty" json:"operation,omitempty" doc:"sum, min, max, avg, last or diff"`
	OperationKey string          `yaml:"operationKey,omitempty" json:"operationKey,omitempty" doc:"internal field on which to perform the operation"`
	TopK         int             `yaml:"topK,omitempty" json:"topK,omitempty" doc:"number of highest incidence to report (default - report all)"`
	BotK         int             `yaml:"botK,omitempty" json:"botK,omitempty" doc:"number of lowest incidence to report (default - report all)"`
	TimeInterval int             `yaml:"timeInterval,omitempty" json:"timeInterval,omitempty" doc:"seconds of data to use to compute the metric"`
}
