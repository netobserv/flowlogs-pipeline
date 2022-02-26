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
	"encoding/json"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type DecodeJson struct {
	PrevRecords []interface{}
}

// Decode decodes input strings to a list of flow entries
// All entries should be saved as strings
func (c *DecodeJson) Decode(in []interface{}) []config.GenericMap {
	out := make([]config.GenericMap, 0)
	for _, line := range in {
		log.Debugf("decodeJson: line = %v", line)
		line2 := []byte(line.(string))
		var decodedLine map[string]interface{}
		err := json.Unmarshal(line2, &decodedLine)
		if err != nil {
			log.Errorf("decodeJson Decode: error unmarshalling a line: %v", err)
			continue
		}
		decodedLine2 := make(config.GenericMap, len(decodedLine))
		for k, v := range decodedLine {
			if v == nil {
				continue
			}
			decodedLine2[k] = v
		}
		out = append(out, decodedLine2)
	}
	c.PrevRecords = in
	return out
}

// NewDecodeJson create a new decode
func NewDecodeJson() (Decoder, error) {
	log.Debugf("entering NewDecodeJson")
	return &DecodeJson{}, nil
}
