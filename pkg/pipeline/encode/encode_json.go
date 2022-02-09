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

package encode

import (
	"encoding/json"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	log "github.com/sirupsen/logrus"
)

type encodeJson struct {
}

// Encode encodes json to byte array
// All entries should be saved as strings
func (e *encodeJson) Encode(inputMetrics []config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for _, metric := range inputMetrics {
		log.Debugf("encodeJson, metric = %v", metric)
		var line []byte
		var err error
		line, err = json.Marshal(metric)
		if err != nil {
			log.Errorf("encodeJson Decode: error marshalling a line: %v", err)
			continue
		}
		out = append(out, line)
	}
	return out
}

// NewEndcodeJson create a new encode
func NewEncodeJson() (Encoder, error) {
	log.Debugf("entering NewEncodeJson")
	return &encodeJson{}, nil
}
