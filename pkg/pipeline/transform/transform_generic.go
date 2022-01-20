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
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Generic struct {
	api.TransformGeneric
}

// Transform transforms a flow to a new set of keys
func (g *Generic) Transform(f config.GenericMap) config.GenericMap {
	log.Debugf("f = %v", f)
	gm := make(config.GenericMap)
	for _, transformRule := range g.Rules {
		log.Debugf("transformRule = %v", transformRule)
		gm[transformRule.Output] = f[transformRule.Input]
	}
	log.Debugf("Transform.GenericMap = %v", gm)
	return gm
}

// NewTransformGeneric create a new transform
func NewTransformGeneric(generic api.TransformGeneric) (Transformer, error) {
	log.Debugf("entering NewTransformGeneric")
	log.Debugf("rules = %v", generic.Rules)
	return &Generic{
		api.TransformGeneric{
			Rules: generic.Rules,
		},
	}, nil
}
