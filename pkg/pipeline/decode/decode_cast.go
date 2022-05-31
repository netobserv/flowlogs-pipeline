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

package decode

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type DecodeCast struct {
	PrevRecords []interface{}
}

// Decode casts input as a list of flow entries
func (c *DecodeCast) Decode(in []interface{}) []config.GenericMap {
	out := make([]config.GenericMap, 0)
	for _, line := range in {
		log.Debugf("decodeCast: line = %v", line)
		out = append(out, config.GenericMap(line.(map[string]interface{})))
	}
	c.PrevRecords = in
	return out
}

// NewDecodeCast create a new decode
func NewDecodeCast() (Decoder, error) {
	log.Debugf("entering NewDecodeCast")
	return &DecodeCast{}, nil
}
