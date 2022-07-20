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

package extract

import (
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract/timebased"
	log "github.com/sirupsen/logrus"
)

type ExtractTimebased struct {
	filters          []timebased.FilterStruct
	recordKeyStructs map[string]*timebased.RecordKeyTable
}

// Extract extracts a flow before being stored
func (et *ExtractTimebased) Extract(entries []config.GenericMap) []config.GenericMap {
	log.Debugf("entering ExtractTimebased Extract")
	nowInSecs := time.Now().Unix()
	for _, entry := range entries {
		log.Debugf("ExtractTimebased Extract, entry = %v", entry)
		timebased.AddEntryToTables(et.recordKeyStructs, entry, nowInSecs)
	}

	// TODO: calculate filters and build return []config.GenericMap
	for _, filter := range et.filters {
		filter.CalculateResults(nowInSecs)
	}
	return nil
}

//  NewExtractTimebased creates a new extractor
func NewExtractTimebased(params config.StageParam) (Extractor, error) {
	log.Debugf("entering NewExtractTimebased")
	var rules []api.TimebasedFilterRule
	if params.Extract != nil && params.Extract.Timebased.Rules != nil {
		rules = params.Extract.Timebased.Rules
	}
	log.Debugf("NewExtractTimebased; rules = %v", rules)

	tmpRecordKeyStructs, tmpFilters := timebased.CreateRecordKeysAndFilters(rules)

	return &ExtractTimebased{
		filters:          tmpFilters,
		recordKeyStructs: tmpRecordKeyStructs,
	}, nil
}
