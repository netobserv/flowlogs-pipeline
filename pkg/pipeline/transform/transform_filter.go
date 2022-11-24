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
	"github.com/sirupsen/logrus"
)

var tlog = logrus.WithField("component", "transform.Filter")

type Filter struct {
	Rules []api.TransformFilterRule
}

// Transform transforms a flow
func (f *Filter) Transform(entry config.GenericMap) (config.GenericMap, bool) {
	tlog.Tracef("f = %v", f)
	outputEntry := entry.Copy()
	for _, rule := range f.Rules {
		tlog.Tracef("rule = %v", rule)
		switch rule.Type {
		case api.TransformFilterOperationName("RemoveField"):
			delete(outputEntry, rule.Input)
		case api.TransformFilterOperationName("RemoveEntryIfExists"):
			if _, ok := entry[rule.Input]; ok {
				return nil, false
			}
		case api.TransformFilterOperationName("RemoveEntryIfDoesntExist"):
			if _, ok := entry[rule.Input]; !ok {
				return nil, false
			}
		case api.TransformFilterOperationName("RemoveEntryIfEqual"):
			if val, ok := entry[rule.Input]; ok {
				if val == rule.Value {
					return nil, false
				}
			}
		case api.TransformFilterOperationName("RemoveEntryIfNotEqual"):
			if val, ok := entry[rule.Input]; ok {
				if val != rule.Value {
					return nil, false
				}
			}
		default:
			tlog.Panicf("unknown type %s for transform.Filter rule: %v", rule.Type, rule)
		}
	}
	return outputEntry, true
}

// NewTransformFilter create a new filter transform
func NewTransformFilter(params config.StageParam) (Transformer, error) {
	tlog.Debugf("entering NewTransformFilter")
	rules := []api.TransformFilterRule{}
	if params.Transform != nil && params.Transform.Filter != nil {
		rules = params.Transform.Filter.Rules
	}
	transformFilter := &Filter{
		Rules: rules,
	}
	return transformFilter, nil
}
