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

package conntrack

import (
	"fmt"
	"math"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

// aggregator represents a single aggregate field in a connection. The aggregated values are stored in the connection
// but managed by the aggregator.
type aggregator interface {
	// addField adds an aggregate field to the connection
	addField(conn connection)
	// update updates the aggregate field in the connection based on the flow log.
	update(conn connection, flowLog config.GenericMap, d direction)
}

type aggregateBase struct {
	inputField  string
	outputField string
	splitAB     bool
	initVal     float64
}

type aSum struct{ aggregateBase }
type aCount struct{ aggregateBase }
type aMin struct{ aggregateBase }
type aMax struct{ aggregateBase }

// TODO: think of adding a more complex operation such as Average Packet Size which involves 2 input fields: Bytes/Packets

// newAggregator returns a new aggregator depending on the output field operation
func newAggregator(of api.OutputField) (aggregator, error) {
	if of.Name == "" {
		return nil, fmt.Errorf("empty name %v", of)
	}
	var inputField string
	if of.Input != "" {
		inputField = of.Input
	} else {
		inputField = of.Name
	}
	aggBase := aggregateBase{inputField: inputField, outputField: of.Name, splitAB: of.SplitAB}
	var agg aggregator
	switch of.Operation {
	case api.ConnTrackOperationName("Sum"):
		aggBase.initVal = 0
		agg = &aSum{aggBase}
	case api.ConnTrackOperationName("Count"):
		aggBase.initVal = 0
		agg = &aCount{aggBase}
	case api.ConnTrackOperationName("Min"):
		aggBase.initVal = math.MaxFloat64
		agg = &aMin{aggBase}
	case api.ConnTrackOperationName("Max"):
		aggBase.initVal = -math.MaxFloat64
		agg = &aMax{aggBase}
	default:
		return nil, fmt.Errorf("unknown operation: %q", of.Operation)
	}
	return agg, nil
}

func (agg *aggregateBase) getOutputField(d direction) string {
	outputField := agg.outputField
	if agg.splitAB {
		switch d {
		case dirAB:
			outputField += "_AB"
		case dirBA:
			outputField += "_BA"
		default:
			log.Panicf("splitAB aggregator %v cannot determine outputField because direction is missing. Check configuration.", outputField)
		}
	}
	return outputField
}

func (agg *aggregateBase) getInputFieldValue(flowLog config.GenericMap) (float64, error) {
	rawValue, ok := flowLog[agg.inputField]
	if !ok {
		return 0, fmt.Errorf("missing field %v", agg.inputField)
	}
	floatValue, err := utils.ConvertToFloat64(rawValue)
	if err != nil {
		return 0, fmt.Errorf("cannot convert %v to float64: %w", rawValue, err)
	}
	return floatValue, nil
}

func (agg *aggregateBase) addField(conn connection) {
	if agg.splitAB {
		conn.addAgg(agg.getOutputField(dirAB), agg.initVal)
		conn.addAgg(agg.getOutputField(dirBA), agg.initVal)
	} else {
		conn.addAgg(agg.getOutputField(dirNA), agg.initVal)
	}
}

func (agg *aSum) update(conn connection, flowLog config.GenericMap, d direction) {
	outputField := agg.getOutputField(d)
	v, err := agg.getInputFieldValue(flowLog)
	if err != nil {
		log.Errorf("error updating connection %x: %v", conn.getHash().hashTotal, err)
		return
	}
	conn.updateAggValue(outputField, func(curr float64) float64 {
		return curr + v
	})
}

func (agg *aCount) update(conn connection, flowLog config.GenericMap, d direction) {
	outputField := agg.getOutputField(d)
	conn.updateAggValue(outputField, func(curr float64) float64 {
		return curr + 1
	})
}

func (agg *aMin) update(conn connection, flowLog config.GenericMap, d direction) {
	outputField := agg.getOutputField(d)
	v, err := agg.getInputFieldValue(flowLog)
	if err != nil {
		log.Errorf("error updating connection %x: %v", conn.getHash().hashTotal, err)
		return
	}

	conn.updateAggValue(outputField, func(curr float64) float64 {
		return math.Min(curr, v)
	})
}

func (agg *aMax) update(conn connection, flowLog config.GenericMap, d direction) {
	outputField := agg.getOutputField(d)
	v, err := agg.getInputFieldValue(flowLog)
	if err != nil {
		log.Errorf("error updating connection %x: %v", conn.getHash().hashTotal, err)
		return
	}

	conn.updateAggValue(outputField, func(curr float64) float64 {
		return math.Max(curr, v)
	})
}
