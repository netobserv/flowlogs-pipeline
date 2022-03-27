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
	maintain  bool
	rules     []api.GenericTransformRule
	mapOfKeys map[string]bool
}

// Transform transforms a flow to a new set of keys
func (g *Generic) Transform(input []config.GenericMap) []config.GenericMap {
	log.Debugf("f = %v", g)
	output := make([]config.GenericMap, 0)
	for _, entry := range input {
		outputEntry := make(config.GenericMap)
		for _, transformRule := range g.rules {
			log.Debugf("transformRule = %v", transformRule)
			outputEntry[transformRule.Output] = entry[transformRule.Input]
		}
		if g.maintain {
			g.MaintainEntrichment(outputEntry, entry)
		}
		log.Debugf("Transform.GenericMap = %v", outputEntry)
		output = append(output, outputEntry)
	}
	return output
}

func (g *Generic) MaintainEntrichment(outputEntry, inputEntry config.GenericMap) {
	log.Debugf("entering MaintainEntrichment")
	for key, value := range inputEntry {
		// if key appears in list of Rules (mapOfKeys), it was already handled
		if g.mapOfKeys[key] {
			continue
		}
		outputEntry[key] = value
	}
}

// NewTransformGeneric create a new transform
func NewTransformGeneric(params config.StageParam) (Transformer, error) {
	log.Debugf("entering NewTransformGeneric")
	log.Debugf("params.Transform.Generic = %v", params.Transform.Generic)
	mapOfKeys := make(map[string]bool)
	rules := params.Transform.Generic.Rules
	for _, rule := range rules {
		mapOfKeys[rule.Input] = true
	}
	transformGeneric := &Generic{
		maintain:  params.Transform.Generic.Maintain,
		rules:     rules,
		mapOfKeys: mapOfKeys,
	}
	log.Debugf("transformGeneric = %v", transformGeneric)
	return transformGeneric, nil
}
