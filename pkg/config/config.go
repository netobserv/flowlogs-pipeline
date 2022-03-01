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

import (
	"encoding/json"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/sirupsen/logrus"
)

type GenericMap map[string]interface{}

var (
	Opt        = Options{}
	PipeLine   []Stage
	Parameters []Param
)

type Options struct {
	PipeLine   string
	Parameters string
	Health     Health
}

type Health struct {
	Port string
}

type Stage struct {
	Name    string
	Follows string
}

type Param struct {
	Name      string
	Ingest    Ingest
	Decode    Decode
	Transform Transform
	Extract   Extract
	Encode    Encode
	Write     Write
}

type Ingest struct {
	Type      string
	File      File
	Collector api.IngestCollector
	Kafka     api.IngestKafka
}

type File struct {
	Filename string
}

type Aws struct {
	Fields []string
}

type Decode struct {
	Type string
	Aws  api.DecodeAws
}

type Transform struct {
	Type    string
	Generic api.TransformGeneric
	Network api.TransformNetwork
}

type Extract struct {
	Type       string
	Aggregates []api.AggregateDefinition
}

type Encode struct {
	Type  string
	Prom  api.PromEncode
	Kafka api.EncodeKafka
}

type Write struct {
	Type string
	Loki api.WriteLoki
}

// ParseConfig creates the internal unmarshalled representation from the Pipeline and Parameters json
func ParseConfig() error {
	logrus.Debugf("config.Opt.PipeLine = %v ", Opt.PipeLine)
	err := json.Unmarshal([]byte(Opt.PipeLine), &PipeLine)
	if err != nil {
		logrus.Errorf("error when reading config file: %v", err)
		return err
	}
	logrus.Debugf("stages = %v ", PipeLine)

	err = json.Unmarshal([]byte(Opt.Parameters), &Parameters)
	if err != nil {
		logrus.Errorf("error when reading config file: %v", err)
		return err
	}
	logrus.Debugf("params = %v ", Parameters)
	return nil
}
