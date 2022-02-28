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
	"strings"
)

var defaultKeys = []string{
	"version",
	"account-id",
	"interface-id",
	"srcaddr",
	"dstaddr",
	"srcport",
	"dstport",
	"protocol",
	"packets",
	"bytes",
	"start",
	"end",
	"action",
	"log-status",
}

type decodeAws struct {
	keyTags []string
}

// Decode decodes input strings to a list of flow entries
func (c *decodeAws) Decode(in []interface{}) []config.GenericMap {
	log.Debugf("entring Decode aws")
	log.Debugf("Decode aws, in = %v", in)
	out := make([]config.GenericMap, 0)
	nItems := len(in)
	log.Debugf("nItems = %d", nItems)
	for lineNum, line := range in {
		lineSlice := strings.Fields(line.(string))
		nFields := len(lineSlice)
		if nFields != len(c.keyTags) {
			log.Errorf("decodeAws Decode: wrong number of fields in line %d", lineNum+1)
			continue
		}
		record := make(config.GenericMap)
		for fieldNum := 0; fieldNum < nFields; fieldNum++ {
			record[c.keyTags[fieldNum]] = lineSlice[fieldNum]
		}
		log.Debugf("record = %v", record)
		out = append(out, record)
	}
	log.Debugf("exiting Decode aws")
	return out
}

// NewDecodeAws create a new decode
func NewDecodeAws(params config.Param) (Decoder, error) {
	log.Debugf("entering NewDecodeAws")
	recordKeys := params.Decode.Aws.Fields
	if len(recordKeys) == 0 {
		recordKeys = defaultKeys
	}
	log.Debugf("recordKeys = %v", recordKeys)
	return &decodeAws{
		keyTags: recordKeys,
	}, nil
}
