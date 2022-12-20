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
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

type connection interface {
	addAgg(fieldName string, initValue float64)
	getAggValue(fieldName string) (float64, bool)
	updateAggValue(fieldName string, newValueFn func(curr float64) float64)
	setExpiryTime(t time.Time)
	getExpiryTime() time.Time
	setNextUpdateReportTime(t time.Time)
	getNextUpdateReportTime() time.Time
	toGenericMap() config.GenericMap
	getHash() totalHashType
	// markReported marks the connection as has been reported. That is, at least one connection record has been emitted
	// for this connection (i.e. newConnection, updateConnection, endConnection).
	// It returns true on the first invocation to indicate the first report. Otherwise, it returns false.
	markReported() bool
	isMatchSelector(map[string]string) bool
}

type connType struct {
	hash                 totalHashType
	keys                 config.GenericMap
	aggFields            map[string]float64
	expiryTime           time.Time
	nextUpdateReportTime time.Time
	isReported           bool
}

func (c *connType) addAgg(fieldName string, initValue float64) {
	c.aggFields[fieldName] = initValue
}

func (c *connType) getAggValue(fieldName string) (float64, bool) {
	v, ok := c.aggFields[fieldName]
	return v, ok
}

func (c *connType) updateAggValue(fieldName string, newValueFn func(curr float64) float64) {
	v, ok := c.aggFields[fieldName]
	if !ok {
		log.Panicf("tried updating missing field %v", fieldName)
	}
	c.aggFields[fieldName] = newValueFn(v)
}

func (c *connType) setExpiryTime(t time.Time) {
	c.expiryTime = t
}

func (c *connType) getExpiryTime() time.Time {
	return c.expiryTime
}

func (c *connType) setNextUpdateReportTime(t time.Time) {
	c.nextUpdateReportTime = t
}

func (c *connType) getNextUpdateReportTime() time.Time {
	return c.nextUpdateReportTime
}

func (c *connType) toGenericMap() config.GenericMap {
	gm := config.GenericMap{}
	for k, v := range c.aggFields {
		gm[k] = v
	}
	// In case of a conflict between the keys and the aggFields, the keys should prevail.
	for k, v := range c.keys {
		gm[k] = v
	}
	return gm
}

func (c *connType) getHash() totalHashType {
	return c.hash
}

func (c *connType) markReported() bool {
	isFirst := !c.isReported
	c.isReported = true
	return isFirst
}

func (c *connType) isMatchSelector(selector map[string]string) bool {
	for k, v := range selector {
		if c.keys[k] != v {
			return false
		}
	}
	return true
}

type connBuilder struct {
	conn *connType
}

func NewConnBuilder() *connBuilder {
	return &connBuilder{
		conn: &connType{
			aggFields:  make(map[string]float64),
			keys:       config.GenericMap{},
			isReported: false,
		},
	}
}

func (cb *connBuilder) Hash(h totalHashType) *connBuilder {
	cb.conn.hash = h
	return cb
}

func (cb *connBuilder) KeysFrom(flowLog config.GenericMap, kd api.KeyDefinition) *connBuilder {
	for _, fg := range kd.FieldGroups {
		for _, f := range fg.Fields {
			cb.conn.keys[f] = flowLog[f]
		}
	}
	return cb
}

func (cb *connBuilder) Aggregators(aggs []aggregator) *connBuilder {
	for _, agg := range aggs {
		agg.addField(cb.conn)
	}
	return cb
}

func (cb *connBuilder) Build() connection {
	return cb.conn
}
