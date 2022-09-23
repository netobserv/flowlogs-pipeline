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
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type DecodeJson struct {
}

// Decode decodes input strings to a list of flow entries
func (c *DecodeJson) Decode(in [][]byte) []config.GenericMap {
	out := make([]config.GenericMap, 0)
	for _, line := range in {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("decodeJson: line = %v", string(line))
		}
		var decodedLine map[string]interface{}
		err := json.Unmarshal(line, &decodedLine)
		if err != nil {
			log.Errorf("decodeJson Decode: error unmarshalling a line: %v", err)
			log.Errorf("line = %s", line)
			continue
		}
		decodedLine2 := make(config.GenericMap, len(decodedLine))
		// flows directly ingested by flp-transformer won't have this field, so we need to add it
		// here. If the received line already contains the field, it will be overridden later
		decodedLine2["TimeReceived"] = time.Now().Unix()
		for k, v := range decodedLine {
			if v == nil {
				continue
			}
			decodedLine2[k] = v
		}
		out = append(out, decodedLine2)
	}
	return out
}

// NewDecodeJson create a new decode
func NewDecodeJson() (Decoder, error) {
	log.Debugf("entering NewDecodeJson")
	return &DecodeJson{}, nil
}
