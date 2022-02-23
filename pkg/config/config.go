/*
 * Copyright (C) 2021 IBM, Inc.
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

package config

type GenericMap map[string]interface{}

var (
	Opt = Options{}
)

type Options struct {
	PipeLine Pipeline
	Health   Health
}

type Health struct {
	Port string
}

type Pipeline struct {
	Ingest    Ingest
	Decode    Decode
	Transform string
	Extract   Extract
	Encode    Encode
	Write     Write
}

type Ingest struct {
	Type      string
	File      File
	Collector string
	Kafka     string
}

type File struct {
	Filename string
}

type Aws struct {
	Fields []string
}

type Decode struct {
	Type string
	Aws  string
}

type Extract struct {
	Type       string
	Aggregates string
}

type Encode struct {
	Type  string
	Prom  string
	Kafka string
}

type Write struct {
	Type string
	Loki string
}
