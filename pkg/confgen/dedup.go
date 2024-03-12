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

package confgen

import (
	"reflect"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	log "github.com/sirupsen/logrus"
)

func (cg *ConfGen) dedupe() {
	cg.transformRules = dedupeNetworkTransformRules(cg.transformRules)
	cg.aggregates.Rules = dedupeAggregateDefinitions(cg.aggregates.Rules)
}

func dedupeNetworkTransformRules(rules api.NetworkTransformRules) api.NetworkTransformRules {
	var dedupeSlice []api.NetworkTransformRule
	for i, rule := range rules {
		if containsNetworkTransformRule(dedupeSlice, rule) {
			// duplicate aggregateDefinition
			log.Debugf("Remove duplicate transformation rule %v at index %v", rule, i)
			continue
		}
		dedupeSlice = append(dedupeSlice, rule)
	}
	return dedupeSlice
}

func containsNetworkTransformRule(slice []api.NetworkTransformRule, rule api.NetworkTransformRule) bool {
	for _, item := range slice {
		if reflect.DeepEqual(item, rule) {
			return true
		}
	}
	return false
}

// dedupeAggregateDefinitions is inefficient because we can't use a map to look for duplicates.
// The reason is that aggregate.AggregateDefinition is not hashable due to its AggregateBy field which is a slice.
func dedupeAggregateDefinitions(aggregateDefinitions api.AggregateDefinitions) api.AggregateDefinitions {
	var dedupeSlice []api.AggregateDefinition
	for i, aggregateDefinition := range aggregateDefinitions {
		if containsAggregateDefinitions(dedupeSlice, &aggregateDefinition) {
			// duplicate aggregateDefinition
			log.Debugf("Remove duplicate AggregateDefinitions %v at index %v", aggregateDefinition, i)
			continue
		}
		dedupeSlice = append(dedupeSlice, aggregateDefinition)
	}
	return dedupeSlice
}

func containsAggregateDefinitions(slice []api.AggregateDefinition, searchItem *api.AggregateDefinition) bool {
	for _, item := range slice {
		if reflect.DeepEqual(item, *searchItem) {
			return true
		}
	}
	return false
}
