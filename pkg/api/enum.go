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

package api

import (
	"log"
	"reflect"
)

type enums struct {
	PromEncodeOperationEnum       PromEncodeOperationEnum
	TransformNetworkOperationEnum TransformNetworkOperationEnum
	KafkaEncodeBalancerEnum       KafkaEncodeBalancerEnum
}

type enumNameCacheKey struct {
	enum      interface{}
	operation string
}

var enumNamesCache = map[enumNameCacheKey]string{}

// GetEnumName gets the name of an enum value from the representing enum struct based on `TagYaml` tag.
func GetEnumName(enum interface{}, operation string) string {
	key := enumNameCacheKey{enum: enum, operation: operation}
	cachedValue, found := enumNamesCache[key]
	if found {
		return cachedValue
	}

	peoEnum := enum
	d := reflect.ValueOf(peoEnum)
	field, found := d.Type().FieldByName(operation)
	if !found {
		log.Panicf("can't find operation %s in enum %v", operation, enum)
		return ""
	}
	tag := field.Tag.Get(TagYaml)

	enumNamesCache[key] = tag
	return tag
}

// GetEnumReflectionTypeByFieldName gets the enum struct `reflection Type` from the name of the struct (using fields from `enums{}` struct).
func GetEnumReflectionTypeByFieldName(enumName string) reflect.Type {
	d := reflect.ValueOf(enums{})
	field, found := d.Type().FieldByName(enumName)
	if !found {
		log.Panicf("can't find enumName %s in enums", enumName)
		return nil
	}

	return field.Type
}
