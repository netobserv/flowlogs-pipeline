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
	"github.com/sirupsen/logrus"
)

var glog = logrus.WithField("component", "transform.Generic")

type Generic struct {
	policy string
	rules  []api.GenericTransformRule
}

// Transform transforms a flow to a new set of keys
func (g *Generic) Transform(entry config.GenericMap) (config.GenericMap, bool) {
	var outputEntry config.GenericMap
	log.Tracef("Transform input = %v", entry)
	if g.policy != "replace_keys" {
		outputEntry = entry.Copy()
	} else {
		outputEntry = config.GenericMap{}
	}
	for _, transformRule := range g.rules {
		outputEntry[transformRule.Output] = entry[transformRule.Input]
	}
	glog.Tracef("Transform output = %v", outputEntry)
	return outputEntry, true
}

// NewTransformGeneric create a new transform
func NewTransformGeneric(params config.StageParam) (Transformer, error) {
	glog.Debugf("entering NewTransformGeneric")
	genConfig := api.TransformGeneric{}
	if params.Transform != nil && params.Transform.Generic != nil {
		genConfig = *params.Transform.Generic
	}
	glog.Debugf("params.Transform.Generic = %v", genConfig)
	rules := genConfig.Rules
	policy := genConfig.Policy
	switch policy {
	case "replace_keys", "preserve_original_keys", "":
		// valid; nothing to do
		glog.Infof("NewTransformGeneric, policy = %s", policy)
	default:
		glog.Panicf("unknown policy %s for transform.generic", policy)
	}
	transformGeneric := &Generic{
		policy: policy,
		rules:  rules,
	}
	glog.Debugf("transformGeneric = %v", transformGeneric)
	return transformGeneric, nil
}
