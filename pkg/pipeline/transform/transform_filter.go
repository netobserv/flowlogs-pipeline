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

package transform

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Filter struct {
	Rules []api.TransformFilterRule
}

// Transform transforms a flow
func (f *Filter) Transform(input []config.GenericMap) []config.GenericMap {
	log.Debugf("f = %v", f)
	output := make([]config.GenericMap, 0)
	for _, entry := range input {
		// copy input entry before transform to avoid alteration on parallel stages
		outputEntry := entry.Copy()
		addToOutput := true
		for _, rule := range f.Rules {
			log.Debugf("rule = %v", rule)
			switch rule.Type {
			case api.TransformFilterOperationName("RemoveField"):
				delete(outputEntry, rule.Input)
			case api.TransformFilterOperationName("RemoveEntryIfExists"):
				if _, ok := entry[rule.Input]; ok {
					addToOutput = false
				}
			case api.TransformFilterOperationName("RemoveEntryIfDoesntExist"):
				if _, ok := entry[rule.Input]; !ok {
					addToOutput = false
				}
			case api.TransformFilterOperationName("RemoveEntryIfEqual"):
				if val, ok := entry[rule.Input]; ok {
					if val == rule.Value {
						addToOutput = false
					}
				}
			case api.TransformFilterOperationName("RemoveEntryIfNotEqual"):
				if val, ok := entry[rule.Input]; ok {
					if val != rule.Value {
						addToOutput = false
					}
				}
			default:
				log.Panicf("unknown type %s for transform.Filter rule: %v", rule.Type, rule)
			}
		}
		if addToOutput {
			output = append(output, outputEntry)
			log.Debugf("Transform.GenericMap = %v", outputEntry)
		}
	}
	return output
}

// NewTransformFilter create a new filter transform
func NewTransformFilter(params config.StageParam) (Transformer, error) {
	log.Debugf("entering NewTransformFilter")
	rules := []api.TransformFilterRule{}
	if params.Transform != nil && params.Transform.Filter != nil {
		rules = params.Transform.Filter.Rules
	}
	transformFilter := &Filter{
		Rules: rules,
	}
	return transformFilter, nil
}
