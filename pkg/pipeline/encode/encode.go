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

package encode

import (
	log "github.com/sirupsen/logrus"
	"github.ibm.com/MCNM/observability/flowlogs2metrics/pkg/config"
)

type encodeNone struct {
}

type Encoder interface {
	Encode(in []config.GenericMap) []interface{}
}

// Encode encodes a flow before being stored
func (t *encodeNone) Encode(in []config.GenericMap) []interface{} {
	out := make([]interface{}, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// NewEncodeNone create a new encode
func NewEncodeNone() (Encoder, error) {
	log.Debugf("entering NewEncodeNone")
	return &encodeNone{}, nil
}
