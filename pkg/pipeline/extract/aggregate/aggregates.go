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

package aggregate

import (
	"fmt"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	log "github.com/sirupsen/logrus"
	"reflect"
)

type Aggregates []Aggregate
type Definitions []api.AggregateDefinition

func (aggregates Aggregates) Evaluate(entries []config.GenericMap) error {
	for _, aggregate := range aggregates {
		err := aggregate.Evaluate(entries)
		if err != nil {
			log.Debugf("Evaluate error %v", err)
			continue
		}
	}

	return nil
}

func (aggregates Aggregates) GetMetrics() []config.GenericMap {
	var metrics []config.GenericMap
	for _, aggregate := range aggregates {
		aggregateMetrics := aggregate.GetMetrics()
		metrics = append(metrics, aggregateMetrics...)
	}

	return metrics
}

func (aggregates Aggregates) AddAggregate(aggregateDefinition api.AggregateDefinition) Aggregates {
	aggregate := Aggregate{
		Definition: aggregateDefinition,
		Groups:     map[NormalizedValues]*GroupState{},
	}

	appendedAggregates := append(aggregates, aggregate)
	return appendedAggregates
}

func (aggregates Aggregates) RemoveAggregate(by api.AggregateBy) (Aggregates, error) {
	for i, other := range aggregates {
		if reflect.DeepEqual(other.Definition.By, by) {
			return append(aggregates[:i], aggregates[i+1:]...), nil
		}
	}
	return aggregates, fmt.Errorf("can't find AggregateBy = %v", by)
}

func NewAggregatesFromConfig(definitions []api.AggregateDefinition) (Aggregates, error) {
	aggregates := Aggregates{}

	for _, aggregateDefinition := range definitions {
		aggregates = aggregates.AddAggregate(aggregateDefinition)
	}

	return aggregates, nil
}
