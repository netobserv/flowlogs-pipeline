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
		log.Debugf("entry.GenericMap = %v", entry)
		outputEntry := make(config.GenericMap)
		if g.policy != "replace_keys" {
			// copy old map to new map
			for key, value := range entry {
				outputEntry[key] = value
			}
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
	genConfig := api.TransformGeneric{}
	if params.Transform != nil && params.Transform.Generic != nil {
		genConfig = *params.Transform.Generic
	}
	log.Debugf("params.Transform.Generic = %v", genConfig)
	rules := genConfig.Rules
	policy := genConfig.Policy
	switch policy {
	case "replace_keys", "preserve_original_keys", "":
		// valid; nothing to do
		log.Infof("NewTransformGeneric, policy = %s", policy)
	default:
		log.Panicf("unknown policy %s for transform.generic", policy)
	}
	transformGeneric := &Generic{
		policy: policy,
		rules:  rules,
	}
	log.Debugf("transformGeneric = %v", transformGeneric)
	return transformGeneric, nil
}
