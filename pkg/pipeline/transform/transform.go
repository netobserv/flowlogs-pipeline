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

package transform

import (
	"encoding/json"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	log "github.com/sirupsen/logrus"
	"os"
)

type Transformer interface {
	Transform(in config.GenericMap) config.GenericMap
}

type transformNone struct {
}

// Transform transforms a flow before being stored
func (t *transformNone) Transform(f config.GenericMap) config.GenericMap {
	return f
}

// NewTransformNone create a new transform
func NewTransformNone() (Transformer, error) {
	log.Debugf("entering  NewTransformNone")
	return &transformNone{}, nil
}

type Definition struct {
	Type    string
	Generic api.TransformGeneric
	Network api.TransformNetwork
}

type Definitions []Definition

const (
	OperationGeneric = "generic"
	OperationNetwork = "network"
	OperationNone    = "none"
)

func GetTransformers() ([]Transformer, error) {
	log.Debugf("entering  GetTransformers")
	var transformers []Transformer
	transformDefinitions := Definitions{}
	transformListString := config.Opt.PipeLine.Transform
	log.Debugf("transformListString = %v", transformListString)
	err := json.Unmarshal([]byte(transformListString), &transformDefinitions)
	if err != nil {
		log.Errorf("error in unmarshalling yaml: %v", err)
		os.Exit(1)
	}
	log.Debugf("transformDefinitions = %v", transformDefinitions)
	for _, item := range transformDefinitions {
		var transformer Transformer
		log.Debugf("item.Type = %v", item.Type)
		switch item.Type {
		case OperationGeneric:
			transformer, _ = NewTransformGeneric(item.Generic)
		case OperationNetwork:
			transformer, _ = NewTransformNetwork(item.Network)
		case OperationNone:
			transformer, _ = NewTransformNone()
		}
		transformers = append(transformers, transformer)
	}
	log.Debugf("transformers = %v", transformers)
	return transformers, nil
}

func ExecuteTransforms(transformers []Transformer, flowEntry config.GenericMap) config.GenericMap {
	for _, transformer := range transformers {
		flowEntry = transformer.Transform(flowEntry)
	}
	return flowEntry
}
