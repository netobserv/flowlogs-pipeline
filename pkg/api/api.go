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

const TagYaml = "yaml"
const TagDoc = "doc"
const TagEnum = "enum"

// Note: items beginning with doc: "## title" are top level items that get divided into sections inside api.md.

type API struct {
	PromEncode       PromEncode       `yaml:"prom" doc:"## Prometheus encode API\nFollowing is the supported API format for prometheus encode:\n"`
	IngestCollector  IngestCollector  `yaml:"collector" doc:"## Ingest collector API\nFollowing is the supported API format for the netflow collector:\n"`
	IngestKafka      IngestKafka      `yaml:"kafka" doc:"## Ingest Kafka API\nFollowing is the supported API format for the kafka ingest:\n"`
	DecodeAws        DecodeAws        `yaml:"aws" doc:"## Aws ingest API\nFollowing is the supported API format for Aws flow entries:\n"`
	TransformGeneric TransformGeneric `yaml:"generic" doc:"## Transform Generic API\nFollowing is the supported API format for generic transformations:\n"`
	TransformNetwork TransformNetwork `yaml:"network" doc:"## Transform Network API\nFollowing is the supported API format for network transformations:\n"`
}
