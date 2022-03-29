/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package transform

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Generic struct {
	policy string
	rules  []api.GenericTransformRule
}

// Transform transforms a flow to a new set of keys
func (g *Generic) Transform(input []config.GenericMap) []config.GenericMap {
	log.Debugf("entering Generic Transform g = %v", g)
	output := make([]config.GenericMap, 0)
	for _, entry := range input {
		outputEntry := entry
		if g.policy == "replace_keys" {
			// start with a clean GenericMap
			outputEntry = make(config.GenericMap)
		}
		log.Debugf("outputEntry = %v", outputEntry)
		for _, transformRule := range g.rules {
			log.Debugf("transformRule = %v", transformRule)
			outputEntry[transformRule.Output] = entry[transformRule.Input]
		}
		log.Debugf("Transform.GenericMap = %v", outputEntry)
		output = append(output, outputEntry)
	}
	return output
}

// NewTransformGeneric create a new transform
func NewTransformGeneric(params config.StageParam) (Transformer, error) {
	log.Debugf("entering NewTransformGeneric")
	log.Debugf("params.Transform.Generic = %v", params.Transform.Generic)
	rules := params.Transform.Generic.Rules
	transformGeneric := &Generic{
		policy: params.Transform.Generic.Policy,
		rules:  rules,
	}
	log.Debugf("transformGeneric = %v", transformGeneric)
	return transformGeneric, nil
}
